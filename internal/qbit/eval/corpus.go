//go:build eval

// Package eval holds the qbit-jay evaluators (E1 context, E2 search, E3
// performance) and the deterministic synthetic corpus they run on. It is built
// only with `-tags eval` and never reaches the release binary. No real user
// data is ever generated or stored here.
package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

// DefaultSeed is the corpus seed used by every reported baseline.
const DefaultSeed uint64 = 20261128

// Anchor is the instant the synthetic timeline starts at. Fixed, so the same
// seed yields the same rows on any day.
var Anchor = time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)

// span is how long the flat (non timeline) corpus is spread over.
const span = 180 * 24 * time.Hour

// Memory is one synthetic observation. Index i depends only on the seed and i,
// never on the corpus size, so the first N memories of any corpus are identical.
type Memory struct {
	Index     int
	Key       string // stable identifier, "m00042"
	Project   string
	Type      string
	TopicKey  string // empty for roughly half of the memories
	Title     string // unique: ends with " (m00042)"
	Content   string
	Lang      string // "en" or "es"
	Comp      int    // index into components
	Topic     int    // index into topics
	Tech      int    // index into technologies
	ODD       bool   // an ODD mirror: topic_key odd/tasks/..., long content
	CreatedAt time.Time
}

// Corpus is a deterministic set of memories.
type Corpus struct {
	Seed     uint64
	Memories []Memory
}

// Generate builds n memories from seed.
func Generate(seed uint64, n int) *Corpus {
	c := &Corpus{Seed: seed, Memories: make([]Memory, n)}
	for i := range n {
		c.Memories[i] = genMemory(seed, i, n)
	}
	return c
}

func newRand(seed uint64, stream ...uint64) *rand.Rand {
	h := seed
	for _, s := range stream {
		h = h*0x9E3779B97F4A7C15 + s + 0x7F4A7C15
	}
	return rand.New(rand.NewPCG(seed, h))
}

func pickWeighted(r *rand.Rand, weights []int) int {
	total := 0
	for _, w := range weights {
		total += w
	}
	x := r.IntN(total)
	for i, w := range weights {
		if x < w {
			return i
		}
		x -= w
	}
	return len(weights) - 1
}

func fill(tpl string, lang string, comp, topic, tech int, n int, unit int) string {
	c, t, u := components[comp].en, topics[topic].en, units[unit].en
	if lang == "es" {
		c, t, u = components[comp].es, topics[topic].es, units[unit].es
	}
	return strings.NewReplacer(
		"{comp}", c, "{topic}", t, "{tech}", technologies[tech],
		"{n}", fmt.Sprint(n), "{unit}", u,
	).Replace(tpl)
}

func upperFirst(s string) string {
	for i, r := range s {
		return strings.ToUpper(string(r)) + s[i+len(string(r)):]
	}
	return s
}

func slug(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), " ", "-")
}

func genMemory(seed uint64, i, total int) Memory {
	r := newRand(seed, uint64(i))
	pw := make([]int, len(projects))
	for k, p := range projects {
		pw[k] = p.weight
	}
	pi := pickWeighted(r, pw)
	tw := make([]int, len(memoryTypes))
	for k, t := range memoryTypes {
		tw[k] = t.weight
	}
	typ := memoryTypes[pickWeighted(r, tw)].name
	lang := "en"
	if r.Float64() < projects[pi].esShare {
		lang = "es"
	}
	comp, topic, tech := r.IntN(len(components)), r.IntN(len(topics)), r.IntN(len(technologies))
	unit := r.IntN(len(units))
	n := 2 + r.IntN(300)
	key := fmt.Sprintf("m%05d", i)
	m := Memory{
		Index: i, Key: key, Project: projects[pi].name, Type: typ, Lang: lang,
		Comp: comp, Topic: topic, Tech: tech,
		CreatedAt: Anchor.Add(span * time.Duration(i) / time.Duration(max(total, 1))),
	}
	isODD := r.IntN(100) < 5
	m.ODD = isODD
	titleTpl := enTitles[typ]
	pl := enPools[typ]
	if lang == "es" {
		titleTpl, pl = esTitles[typ], esPools[typ]
	}
	m.Title = upperFirst(fill(titleTpl, lang, comp, topic, tech, n, unit)) + " (" + key + ")"
	switch {
	case isODD:
		m.Type = "decision"
		m.TopicKey = fmt.Sprintf("odd/tasks/%s-%d", slug(components[comp].en), i)
		if lang == "es" {
			m.Title = fmt.Sprintf("Tarea ODD: %s (%s)", components[comp].es, key)
		} else {
			m.Title = fmt.Sprintf("ODD task: %s (%s)", components[comp].en, key)
		}
		m.Content = oddContent(r, lang, comp, tech, key)
	default:
		if r.IntN(100) < 55 {
			m.TopicKey = fmt.Sprintf("%s/%s-%d", typ, slug(components[comp].en), i)
		}
		parts := []string{
			fill(pl.open[r.IntN(len(pl.open))], lang, comp, topic, tech, n, unit),
			fill(pl.detail[r.IntN(len(pl.detail))], lang, comp, topic, tech, n, unit),
			fill(pl.close[r.IntN(len(pl.close))], lang, comp, topic, tech, n, unit),
		}
		m.Content = strings.Join(parts, " ")
	}
	return m
}

// oddContent builds a long mirror of an ODD task document (a checklist).
func oddContent(r *rand.Rand, lang string, comp, tech int, key string) string {
	var b strings.Builder
	if lang == "es" {
		fmt.Fprintf(&b, "# Tarea ODD: %s\n\nTecnología: %s. Referencia %s.\n\n", components[comp].es, technologies[tech], key)
	} else {
		fmt.Fprintf(&b, "# ODD task: %s\n\nTechnology: %s. Reference %s.\n\n", components[comp].en, technologies[tech], key)
	}
	lines := 25 + r.IntN(35)
	for k := range lines {
		t := r.IntN(len(topics))
		if lang == "es" {
			fmt.Fprintf(&b, "- [ ] T%d: %s en %s (%s)\n", k+1, topics[t].es, components[comp].es, technologies[r.IntN(len(technologies))])
		} else {
			fmt.Fprintf(&b, "- [ ] T%d: %s in the %s (%s)\n", k+1, topics[t].en, components[comp].en, technologies[r.IntN(len(technologies))])
		}
	}
	return b.String()
}

// Revision is the content of a memory after its rev-th update (rev >= 1).
func (m Memory) Revision(rev int) string {
	r := newRand(DefaultSeed, uint64(m.Index), uint64(rev), 99)
	n := 2 + r.IntN(300)
	unit := r.IntN(len(units))
	if m.Lang == "es" {
		return fmt.Sprintf("%s\n\nActualización %d: el límite pasó a %d %s tras revisar %s.", m.Content, rev, n, units[unit].es, topics[r.IntN(len(topics))].es)
	}
	return fmt.Sprintf("%s\n\nUpdate %d: the limit moved to %d %s after reviewing %s.", m.Content, rev, n, units[unit].en, topics[r.IntN(len(topics))].en)
}

// Fingerprint hashes the first n memories (project, type, topic key, title and
// content). The E2 fixture stores it so a generator change is detected.
func (c *Corpus) Fingerprint(n int) string {
	h := sha256.New()
	for _, m := range c.Memories[:min(n, len(c.Memories))] {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00", m.Key, m.Project, m.Type, m.TopicKey, m.Title, m.Content)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ─── Timeline ────────────────────────────────────────────────────────────────

// Prompt is one user prompt of a session.
type Prompt struct {
	At   time.Time
	Text string
}

// Save is the creation of a memory inside a session.
type Save struct {
	Mem int
	At  time.Time
}

// Revise is the update of an existing memory (a version created during the
// session) through its topic_key.
type Revise struct {
	Mem     int
	At      time.Time
	Content string
}

// SessionPlan is one synthetic session: a span of time in one project with
// prompts, new memories and revisions of earlier memories.
type SessionPlan struct {
	ID      string
	Project string
	Start   time.Time
	End     time.Time
	Ended   bool
	Prompts []Prompt
	Saves   []Save
	Revises []Revise
	Summary string
	Lang    string
}

// recentWindow and recencyBias shape which earlier memories a session revisits:
// with probability recencyBias one of the last recentWindow memories of the
// project, otherwise any earlier memory that has a topic_key. They are
// parameters of the synthetic world, not measurements of real behaviour.
const (
	recentWindow = 20
	recencyBias  = 0.45
)

// Plan lays the corpus out as sessions per project. Sessions of one project
// never overlap; sessions of different projects may. Memory creation times of
// a planned corpus come from the plan (Save.At), not from Memory.CreatedAt.
func Plan(c *Corpus) []SessionPlan {
	byProject := map[string][]int{}
	var order []string
	for _, m := range c.Memories {
		if _, ok := byProject[m.Project]; !ok {
			order = append(order, m.Project)
		}
		byProject[m.Project] = append(byProject[m.Project], m.Index)
	}
	var plans []SessionPlan
	for pi, proj := range order {
		r := newRand(c.Seed, 7001, uint64(pi))
		cursor := Anchor.Add(time.Duration(r.IntN(72*60)) * time.Minute)
		idxs := byProject[proj]
		var created []int // memories of this project created so far, in order
		revCount := map[int]int{}
		for pos, sn := 0, 0; pos < len(idxs); sn++ {
			size := 3 + r.IntN(7)
			end := min(pos+size, len(idxs))
			dur := time.Duration(30+r.IntN(150)) * time.Minute
			sp := SessionPlan{
				ID: fmt.Sprintf("ses-%s-%05d", proj, sn), Project: proj,
				Start: cursor, End: cursor.Add(dur), Ended: r.IntN(100) < 95,
			}
			first := c.Memories[idxs[pos]]
			sp.Lang = first.Lang

			var revIdx []int
			if len(created) > 0 && r.Float64() < 0.55 {
				for range 1 + r.IntN(2) {
					var pool []int
					if r.Float64() < recencyBias {
						pool = created[max(0, len(created)-recentWindow):]
					} else {
						pool = created
					}
					// Walk from a random start to the first memory with a topic_key.
					off := r.IntN(len(pool))
					for k := range pool {
						cand := pool[(off+k)%len(pool)]
						if c.Memories[cand].TopicKey != "" && !contains(revIdx, cand) {
							revIdx = append(revIdx, cand)
							break
						}
					}
				}
			}
			slots := (end - pos) + len(revIdx) + 2
			at := func(k int) time.Time { return sp.Start.Add(dur * time.Duration(k) / time.Duration(slots)) }
			k := 1
			for _, mi := range idxs[pos:end] {
				sp.Saves = append(sp.Saves, Save{Mem: mi, At: at(k)})
				k++
			}
			for _, mi := range revIdx {
				revCount[mi]++
				sp.Revises = append(sp.Revises, Revise{Mem: mi, At: at(k), Content: c.Memories[mi].Revision(revCount[mi])})
				k++
			}
			// Prompts: most sessions have 1-3; a few none.
			if r.IntN(100) < 95 {
				first := promptText(r, c, sp, revIdx, idxs[pos])
				sp.Prompts = append(sp.Prompts, Prompt{At: sp.Start.Add(time.Minute), Text: first})
				for range r.IntN(3) {
					sp.Prompts = append(sp.Prompts, Prompt{At: at(1 + r.IntN(slots-1)), Text: promptText(r, c, sp, nil, idxs[pos+r.IntN(end-pos)])})
				}
			}
			sp.Summary = summaryText(c, sp, idxs[pos])
			plans = append(plans, sp)
			created = append(created, idxs[pos:end]...)
			pos = end
			cursor = sp.End.Add(time.Duration(30+r.IntN(690)) * time.Minute)
		}
	}
	return plans
}

func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// promptText: half of the first prompts of a session that revisits a memory
// name its component and topic, as a person resuming work would.
func promptText(r *rand.Rand, c *Corpus, sp SessionPlan, revIdx []int, about int) string {
	m := c.Memories[about]
	if len(revIdx) > 0 && r.IntN(2) == 0 {
		m = c.Memories[revIdx[0]]
	}
	if sp.Lang == "es" {
		return fmt.Sprintf("Sigamos con %s: %s.", components[m.Comp].es, topics[m.Topic].es)
	}
	return fmt.Sprintf("Let's continue with the %s: %s.", components[m.Comp].en, topics[m.Topic].en)
}

func summaryText(c *Corpus, sp SessionPlan, about int) string {
	m := c.Memories[about]
	if sp.Lang == "es" {
		return fmt.Sprintf("Sesión sobre %s en %s con %d memorias nuevas.", components[m.Comp].es, sp.Project, len(sp.Saves))
	}
	return fmt.Sprintf("Session on the %s in %s with %d new memories.", components[m.Comp].en, sp.Project, len(sp.Saves))
}
