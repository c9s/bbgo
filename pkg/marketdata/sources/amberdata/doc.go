// Package amberdata reads historical market data from AmberData.
//
// It exists for one reason: L2. data.binance.vision publishes no order book
// diffs and no price-level snapshots, so a vendor feed is the only way to
// backtest against a real book over a historical range rather than a range
// someone recorded themselves.
//
// # Developing without a key
//
// AmberData has no free tier, no sandbox and no demo key, and every request
// without a valid key returns an AWS API Gateway 403. This package was
// nevertheless written and tested offline, because two things are public:
//
//   - The complete OpenAPI 3.1 specifications. Every documentation page at
//     docs.amberdata.io has a .md twin with the spec inline, including
//     parameter enums, defaults, required-ness, response schemas and response
//     examples. The request builders and response types here come from those,
//     and the examples are the fixtures the decoders are tested against.
//   - Sample Parquet files from the CloudSync bulk product, readable with no
//     credentials at https://amberdata-samples.s3.amazonaws.com/ — including
//     real Binance USDⓈ-M order book updates. That is ground truth for the
//     field semantics no specification pins down.
//
// # UNVERIFIED
//
// The following could not be checked without credentials. Each is reachable
// through the Client interface, so correcting one is a single-file change.
//
//   - The error envelope for an authenticated failure. A 403 is confirmed to
//     arrive in the API Gateway shape, which does not match the documented
//     envelope; whether 400, 404 and 429 use the documented shape is untested,
//     so APIError accepts both.
//   - Rate limit behaviour: whether a 429 carries Retry-After or any
//     rate-limit headers is undocumented, so the backoff is a guess.
//   - Whether cursors expire, and whether a cursor URL preserves the original
//     query parameters. The client re-issues it verbatim on the assumption
//     that it does.
//   - Whether responses are always gzipped, given Accept-Encoding is
//     documented as required.
//   - Whether order-book-events returns full depth or top-N for Binance
//     USDⓈ-M, what maxLevel does exactly, and whether snapshots interleave with
//     events.
//   - Whether the paid CloudSync files are complete. The public sample is not:
//     across its 136,669 Binance order book events the Binance contiguity
//     relation holds only about 0.1% of the time, so the sample is decimated,
//     which is unsurprising for a free sample of a paid product. A gap-free
//     backtest cannot be assumed until a real file is checked.
//   - Whether the REST feed exposes Binance's first-update id at all. The
//     CloudSync files do, as metadata.firstUpdateId, and the cloudsync package
//     uses it to populate Event.PrevSeq so contiguity is provable. The REST
//     response has no equivalent field, so the REST path leaves PrevSeq zero
//     and a book built from it validates as monotonic-but-unverified.
//   - Whether Binance futures `volume` is denominated in the base asset or in
//     contracts. BitMEX is documented to report contracts; Binance is not
//     stated either way. The exchanges/reference endpoint carries
//     contractSize and underlyingToPositionMultiplier, which settles it once a
//     key exists.
//
// # Known documentation inconsistencies
//
//   - The trades endpoint's page warns that the maximum range is 731 days,
//     while its startDate parameter description says the default window is one
//     hour with a maximum range of one hour. The former is treated as the real
//     limit and the latter as a copy-paste artifact.
//   - The timeFormat enum includes a literal "ms*" value, which is a typo for
//     the documented default marker. The client sends "milliseconds".
package amberdata
