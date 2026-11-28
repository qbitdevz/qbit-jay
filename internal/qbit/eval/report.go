//go:build eval

package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

// Meta identifies a run so two reports can be compared honestly.
type Meta struct {
	Label     string `json:"label"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	CPUs      int    `json:"cpus"`
	Seed      uint64 `json:"seed"`
	Sizes     []int  `json:"sizes"`
	Date      string `json:"date_utc"`
}

// Report is the full output of one evaluation run.
type Report struct {
	Meta Meta       `json:"meta"`
	E1   []E1Result `json:"e1"`
	E2   []E2Result `json:"e2"`
	E3   []E3Result `json:"e3"`
}

// WriteJSON stores the report at path.
func (r *Report) WriteJSON(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func pct(v float64) string { return fmt.Sprintf("%.1f%%", v*100) }

// Print renders the results as text tables.
func (r *Report) Print(w io.Writer) {
	m := r.Meta
	_, _ = fmt.Fprintf(w, "\nqbit-jay evaluators: %s, commit %s, %s %s/%s, %d CPUs, seed %d, %s\n",
		m.Label, m.Commit, m.GoVersion, m.OS, m.Arch, m.CPUs, m.Seed, m.Date)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "\nE1 Context (hit rate = sessions with >=1 needed memory in the context)")
	_, _ = fmt.Fprintln(tw, "source\tsize\tsessions\tneeded/session\tbudget\thit rate\trecall\tmean bytes")
	for _, e := range r.E1 {
		for _, row := range e.Rows {
			b := fmt.Sprint(row.Budget)
			if row.Budget == 0 {
				b = "unbounded"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%d\t%d\t%.2f\t%s\t%s\t%s\t%.0f\n", e.Source, e.Size, e.Sessions, e.MeanNeeded, b, pct(row.Hit), pct(row.Recall), row.MeanBytes)
		}
	}
	_ = tw.Flush()

	_, _ = fmt.Fprintln(w, "\nE2 Search (real SearchPreviewsContext, project filter, limit 10)")
	fmt.Fprintln(tw, "size\tmode\tgroup\tqueries\trecall@5\trecall@10\tMRR\tzero-result")
	for _, e := range r.E2 {
		row := func(group string, x E2Metrics) {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\t%s\t%.3f\t%s\n", e.Size, e.MatchMode, group, x.Queries, pct(x.RecallAt5), pct(x.RecallAt10), x.MRR, pct(x.ZeroResult))
		}
		row("overall", e.Overall)
		for _, k := range SortedKeys(e.ByKind) {
			row("kind:"+k, e.ByKind[k])
		}
		for _, k := range SortedKeys(e.ByDirection) {
			row(k, e.ByDirection[k])
		}
	}
	_ = tw.Flush()

	fmt.Fprintln(w, "\nE3 Performance (store level; MCP latency, startup and RSS: scripts/qbit/e3latency)")
	fmt.Fprintln(tw, "size\tload s\tdb MiB\tsearch p50 ms\tsearch p95 ms\tcontext p50 ms\tcontext p95 ms")
	for _, e := range r.E3 {
		fmt.Fprintf(tw, "%d\t%.1f\t%.1f\t%.2f\t%.2f\t%.2f\t%.2f\n", e.Size, e.LoadSeconds, float64(e.DBBytes)/(1<<20), e.SearchP50ms, e.SearchP95ms, e.ContextP50ms, e.ContextP95ms)
	}
	_ = tw.Flush()
}
