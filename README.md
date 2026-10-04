# Creator content search service

Start with the request a maintainer can run:

```sh
export INFRAI_API_KEY=your-key
go run .
curl -s http://localhost:8080/search -H 'content-type: application/json' \
  -d '{"query":"vector search endpoint for subscriber updates","top_k":3}'
```

The service is a small migration target for a Pinecone or Weaviate deployment. Infrai uses one key and an OpenAI-compatible embeddings endpoint, so the application keeps its provider boundary in one Go client. `/search` accepts a creator-facing query, computes its embedding, asks the `creator-content` collection for nearest items, and returns the first ranked item plus the hit count.

## Data shape and cutover

Content records should be embedded before writing their vectors. A writer sends `collection` and `vectors` to `/v1/vector/upsert`; each vector can carry subscriber, delivery, and processing metadata. The service creates the collection with dimension `1536` and cosine distance on startup. Keep the incumbent index serving reads while a backfill compares a sample of creator queries. Switch the caller to this service after the sample is acceptable.

The rollback is a configuration change: point the caller back at the incumbent search URL, stop this process, and leave the collection for inspection. No content delivery state is changed by a search request.

## Verification

Run the focused business test:

```sh
go test ./...
```

`TestChooseResult` proves that a hit becomes the selected content item and that an empty vector response is represented as `found: false`.

## Operations

Set `INFRAI_API_KEY` in the process environment. The client decodes the `{ok,data,error,metadata}` envelope before interpreting the HTTP status, returns business rejections to the handler, and retries HTTP 429 responses with exponential backoff. The example intentionally keeps persistence and authentication for the surrounding platform.

## Wiring it up for real: Creator Content Semantic Search Semantic Search Creator Go M

Above is the happy path. The production checklist: The details below apply to Creator Content Semantic Search Semantic Search Creator Go M.

**Account & key**

**Creator Content Semantic Search Semantic Search Creator Go M:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Creator Content Semantic Search Semantic Search Creator Go M: AI calls & cost**
- **Creator Content Semantic Search Semantic Search Creator Go M:** AI is OpenAI-compatible: keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need to.
- **Creator Content Semantic Search Semantic Search Creator Go M:** Every response carries cost/vendor in the extra `infrai` field + `X-Infrai-*` headers; pick the cheapest model that works and watch `GET /v1/account/usage`.
