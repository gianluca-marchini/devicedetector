# Changelog

## v1.2.0

Faster, deterministic and safe for concurrent use. The public API is
unchanged (only additions, checked with `apidiff`), the format of the YAML
regex files is unchanged.

### Requirements

- Go 1.27 (`go 1.27.0` in go.mod).
- `github.com/dlclark/regexp2` v1.12.0 (was v1.2.0).

### Fixes

**Results depending on the order of a Go map.** When more entries of a YAML
mapping matched a user agent, v1.1.1 returned a random one, different from
one call to the next. Like in Matomo, the entries are now applied in the
order of the YAML file:

- the brands of the device files (`mobiles.yml`, `televisions.yml`, ...):
  e.g. `Galaxy Nexus` was Google or Samsung, `Pulp (iPad)` a smartphone or a
  tablet;
- the vendor fragments (`vendorfragments.yml`);
- the engine versions of a browser (`browsers.yml`, e.g. Opera
  `3.5: Elektra, 7: Presto, 15: Blink`): Opera 12 was Presto or Elektra,
  Opera 114 on Windows Blink or Elektra.

`DeviceParserAbstract.Regexes` and `Engine.Versions` are still maps; the
order is kept in unexported fields filled when the YAML file is decoded.

**Fidelity with Matomo.**

- HbbTV: the regex of the HbbTV fragment was only set when loading
  `televisions.yml` failed. With the empty regex every user agent went
  through `televisions.yml`, whose regexes include `Samsung`, `LGE`, `Sony`,
  `TCL`, ...: 497 phones and tablets of the fixtures were TVs (e.g. every
  user agent with `SamsungBrowser`). Now only the user agents with
  `HbbTV/x.y` are parsed with `televisions.yml`, and they are always TVs,
  even when no brand matches.
- Android device type: the `Android; Tablet;` fragment was never tested (the
  `Android; Mobile;` fragment was tested twice), so Firefox for Android
  (`Android 14; Mobile; ...`) was a tablet.

With these fixes the output matches all the 10,223 non-bot fixtures on every
field (os, client, engine, device type, brand, model); v1.1.1 differed on
1,426 fields. `TestFixtures` now checks all of them.

**Data races** (found with the new `TestParseConcurrent` and `-race`):

- the regexes of `browsers.yml`, `oss.yml` and the HbbTV regex were compiled
  lazily on first use by concurrent parses: everything is compiled at load
  time;
- the aggregated regexes of `PreMatch` were built lazily without
  synchronization: they are built at load time, with a `sync.Once` fallback
  for parsers not built with `Load`;
- `Browser.BuildEngineVersion` wrote a map shared by all the parses
  (possible `fatal error: concurrent map writes`): the map is filled by
  `Load` and only read afterwards;
- `ParseBot` called `DiscardDetails` at every parse: the flag is now an
  `atomic.Bool`.

**Cache.** `Add` on a key already cached (two goroutines parsing the same
user agent) appended the key to `key_list` again; evicting the duplicate
then deleted from the map an entry still in the list. The value is now
replaced in place and every key is in `key_list` once.
`NewDeviceDetector(dir, nil)` disables the cache instead of panicking.

### Performance

Measured on the corpus of the benchmarks (one fixture user agent every 10
plus `testdata/extra_user_agents.txt`), cache disabled, configuration of
intelligence-middleware (`SkipBotDetection`, `DiscardBotInformation`),
Go 1.27.2, Intel i7-10700 (8 cores, 16 threads), v1.1.1 and v1.2.0 measured
in the same session (5 and 3 runs, medians).

| | v1.1.1 | v1.2.0 |
|---|---:|---:|
| `BenchmarkParse` (1 thread) | 10.6 ms/op | 57 µs/op (×186) |
| `BenchmarkParseParallel` (16 threads), throughput | 2.15 ms/op | 8.1 µs/op (×265) |
| `BenchmarkParseParallel`, latency of one parse | 34.4 ms | 130 µs (×265) |
| memory per parse | 360 KB, 806 allocs | 2.7 KB, 45 allocs |
| `NewDeviceDetector` | 60 ms | 160 ms |
| heap of a detector after load / after parsing the corpus | 8.9 / 28.6 MB | 13.4 / 29.5 MB |

The results never depend on the fast paths: they only decide whether and
with which engine a regex is executed.

- **Literal prefilters.** For every regex the strings that a match must
  contain are computed from the syntax tree (in conjunctive form: one string
  per group, e.g. `Android.*Z1([234])0[);/ ]` needs `android` and one of
  `z120)`, `z130/`, ...). Lower-casing is the one of regexp2 with
  `IgnoreCase` (`unicode.ToLower` rune by rune), and `\s`, `\w`, `\d` are
  converted with their Unicode meaning, so a prefilter is always a necessary
  condition, for any input. A regex whose prefilter fails is not executed.
- **List indexes.** The first group of strings of every regex of a list
  (brands, models, browsers, operating systems, clients, bots) is indexed by
  its first bytes: one scan of the user agent gives the regexes that can
  match.
- **Standard library engine.** A regex without constructs whose meaning
  differs between the engines (lookarounds, backreferences, named groups,
  `\p`, class subtraction, ...) is executed with `regexp` (RE2) when the user
  agent is printable ASCII, where the two engines give the same matches and
  groups; otherwise with regexp2. Regexes with lookarounds get an RE2
  "superset" (lookarounds removed) used as a filter. RE2 regexes are
  compiled on first use.
- **Alternatives.** Long regexes (`A|B|...`) are filtered alternative by
  alternative, each with its own prefilter and engine; the aggregated
  `PreMatch` regexes are evaluated as "any regex of the list matches". Both
  are done only when no regex has backreferences, inline options or
  conditionals, which could make the alternation differ from the union.
- **Fewer executions.** Groups are only extracted when the YAML item uses
  `$n`; the user agent is prepared once per parse (`parser.UserAgent`) and
  shared by the built-in parsers; brand short codes and name lookups are
  precomputed instead of scanning maps.

Steps (BenchmarkParse, single thread, each on top of the previous one;
measured at different times, about ±10%):

| step | ns/op |
|---|---:|
| v1.1.1 | 11,000,000 |
| fixes (YAML order, races, cache), user agent prepared once per parser | 9,900,000 |
| regexp2 v1.12.0 | 10,650,000 |
| literal prefilter (any of the required strings) | 755,000 |
| standard library engine | 574,000 |
| prefilter in conjunctive form, bigram bitmap | 304,000 |
| aggregated regexes evaluated as a union | 150,000 |
| alternatives of long regexes, common tokens ranked last | 92,000 |
| list indexes | 62,000 |
| groups only when used, precomputed model brands | 55,000 |
| trigram buckets, RE2 compiled on first use | ~55,000 |
| HbbTV and Android fixes (final, same session as v1.1.1: 10,600,000) | 57,000 |

Evaluated and not kept: lowering the length from which alternatives are
split (no gain); indexing only long model lists (no memory saved).
`regexp2cg` (code generation) does not apply, the regexes are loaded at run
time from the YAML directory.

### Equivalence checks

- `testdata/snapshot.json.gz`: complete output (os, client, device, brand,
  model, device type, mobile/desktop, bots with and without details) for the
  10,743 user agents of the corpus; `TestSnapshotEquivalence` compares the
  current code with it.
- `TestRegexFastPathsEquivalence` (package parser): every pattern of the
  YAML files (6,406, aggregated ones included) against regexp2 on a sample
  of the corpus plus upper-case, lower-case and non-ASCII variants (match and
  every group). `DD_EXHAUSTIVE=1` uses the whole corpus: 128.6 million
  checks, no difference.
- `TestIndexCandidates`, `TestAggregateEquivalence`: the indexes and the
  union evaluation never miss a match.

### Output changes

`docs/v1.2.0-output-changes.tsv` lists the 810 user agents of the corpus
whose output changes (all the values observed with v1.1.1 in 5 runs, the
v1.2.0 value, the fixture value and the entries matching the user agent):

| cause | user agents | v1.2.0 = fixture | no fixture / bot fixture |
|---|---:|---:|---:|
| YAML order | 274 | 272 | 2 |
| HbbTV / Android fragment | 528 | 523 | 5 |
| both | 8 | 8 | 0 |

In no case v1.1.1 gave the fixture value and v1.2.0 does not.

### API additions

- `parser.UserAgent`, `parser.NewUserAgent`: a user agent prepared for
  matching (create one per parse, not safe for concurrent use).
- `parser.Regex`, `parser.NewRegex`: a case-insensitive regexp2 regex with
  the fast paths; `parser.UserAgentPattern`.
- `parser.Index`, `parser.NewIndex`, `parser.Candidates`.
- `(*Regular).MatchUA`, `(*Regular).IsMatchUA`.
- `ParseUserAgent(*parser.UserAgent)` on the built-in parsers (`Mobile`,
  `HbbTv`, `Console`, `Car`, `Camera`, `PortableMediaPlayer`, `Browser`,
  `FeedReader`, `MobileApp`, `MediaPlayer`, `Pim`, `Library`, `Oss`,
  `VendorFragments`). `DeviceDetector` uses it only for these exact types:
  custom parsers, also those embedding a built-in one, still get `Parse`.
- `(*client.Engine).UnmarshalYAML` (keeps the order of `versions`).

A `Regular` built by the caller must be compiled (`Compile`) before being
shared by goroutines; all the regexes loaded by the package are.
