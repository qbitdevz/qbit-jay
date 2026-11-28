# E2 query fixture

`e2_queries.json` is the ground truth of evaluator E2 (search): 450 synthetic
queries, three for each of the first 150 memories of the synthetic corpus
(`Generate(DefaultSeed, n)`, seed 20261128, Spanish and English memories).

All content is synthetic. No real user data is involved (spec 10.3).

## How it was generated

Once, by an LLM (Claude, working as the coding agent of task T0.8), not by a
template. The 150 memories (title and content) were printed with
`TestDumpE2Memories`, read, and three queries were written per memory:

| kind | language | what it is |
| --- | --- | --- |
| `same` | the memory's | a natural-language paraphrase, as a person would ask |
| `kw` | the memory's | two to four keywords, as an agent would search |
| `cross` | the other one | a natural-language query in the other language: Spanish against an English memory, English against a Spanish one |

So half of the `cross` queries are Spanish queries against English memories,
as spec 10.2 requires. The memory key (`m00042`) names the target.

`corpus_fingerprint` is a SHA-256 over those 150 memories. If the generator
changes, `LoadFixture` fails and the queries must be rewritten for the new
memories; do not just update the fingerprint.

## Regenerating

```sh
QBIT_JAY_EVAL_DUMP=/tmp/memories.txt \
  go test -tags eval -run TestDumpE2Memories ./internal/qbit/eval/
# write /tmp/queries.txt: one line per memory, "key|same|keywords|cross"
QBIT_JAY_EVAL_BUILD_FIXTURE=/tmp/queries.txt \
  go test -tags eval -run TestBuildFixture ./internal/qbit/eval/
```

A sample can be reviewed by eye: open the JSON and compare a few queries with
their memories in the dump.
