# saaslb-backend

Go API for **saas leaderboards**: a pay-to-rank board of SaaS sites.

There is no bidding. You pay a whole-dollar amount (or `$0` to list for free). That payment is added to a ledger. Rank is whoever has paid the most in the selected rolling window.

Money is collected through [Dodo Payments](https://docs.dodopayments.com) hosted checkout. The frontend (`saaslb-frontend`) is a React app that calls this API.

---

## 1. Idea

1. A site is uniquely identified by its URL (`listing_key`).
2. Anyone can list at `$0`.
3. Anyone already listed can pay at least `$1` more. Each payment is **this payment**, not a target bid or a “raise to beat X.”
4. Four boards, all computed from the same ledger:
   - **Daily** — last 24 hours
   - **Weekly** — last 7 days
   - **Monthly** — last 30 days (frontend default)
   - **All time** — every paid checkout since the listing went live
5. Same paid amount in a window: whoever listed **first** stays ahead. A brand-new listing therefore loses ties.
6. There is **no calendar-month reset**. Nothing zeros on the 1st of the month. Windows roll continuously.

---

## 2. Layout

```
cmd/api              HTTP server
internal/config      .env
internal/domain      Rules: URLs, money, rank, categories
internal/store       MongoDB
internal/httpapi     JSON routes
internal/dodo        Dodo checkout + webhook verify
internal/metadesc    Fetch tagline + favicon from the listed site
```

The API listens on `http://localhost:8080`. Point the frontend at it with the Vite `/api` proxy, or set `VITE_API_URL`.

---

## 3. Database

MongoDB database `saaslb` (override with `MONGODB_DB`). Six collections.

Rank, windowed dollars, and windowed clicks are **not stored on the listing**. They are aggregated from `checkouts` and `clicks` on every board read.

### `products` — the listing

One document per site. This is identity and display, plus two running counters.

| Field | What it is |
| --- | --- |
| `_id` | `prd_<slug>` |
| `slug` | Unique URL slug |
| `listing_key` | Canonical host + path. Same URL always hits this row. |
| `website_url` | `https://…` used for outbound clicks |
| `name`, `tagline`, `icon_url`, `categories` | Card copy. Tagline/icon are pulled from the site. |
| `bid_cents` | **All-time paid total** on this listing (legacy field name). Not a bid. Used as a fallback if checkout aggregation is missing. |
| `clicks` | All-time click counter. Windowed clicks come from `clicks`. |
| `created_at` | Tie-break: earlier listing wins. |
| `updated_at` | Last payment or meta refresh |
| `last_checkout_id` | Last fulfilled checkout. Compare-and-set so two payments cannot both apply from the same stale total. |
| `meta_refreshed_at` | 10s cooldown for manual tagline/icon refresh |
| `period` | Leftover from an old monthly reset. Unused. |

Indexes: unique `slug`, unique `listing_key`.

### `checkouts` — the money ledger

Every attempt to pay (including `$0`) is a checkout. Rank is the sum of `paid_cents` where `status` is `paid` and `paid_at` falls inside the window.

| Field | What it is |
| --- | --- |
| `_id` | `chk_…` |
| `listing_key`, `website_url`, `name`, `categories` | Who this payment is for |
| `amount_cents` / `paid_cents` | Same number: **this payment** |
| `status` | `pending` → `paid` or `failed` |
| `paid_at` | When it counted. Window filters use this (fallback: `created_at`). |
| `product_id` | Set on fulfill. Groups the ledger onto a listing. |
| `existing_product_id` | Set if this URL was already listed |
| `session_id` / `payment_id` | Dodo ids, unique when present |

### `clicks` — outbound click events

One row per counted click. Kept forever (no TTL).

| Field | What it is |
| --- | --- |
| `product_id` | Listing |
| `visitor_id` | Browser id from the frontend |
| `dedup_key` | `productId:visitorId:YYYY-MM-DDTHH` (UTC hour). Unique, so one counted click per visitor per listing per hour. |
| `created_at` | Window filters |

### `webhook_events`

Dodo webhook ids. Insert-once so a retried `payment.succeeded` does not pay twice.

### `visitors` + `meta`

Presence: unique visitor ids, all-time visit count, launch date. Not used for rank.

---

## 4. How rank is computed

On `GET /api/products` (and any other product read):

1. Load every `products` document.
2. Aggregate paid checkouts into `paidDailyCents`, `paidWeeklyCents`, `paidMonthlyCents`, `paidAllTimeCents`, plus `lastPaidAt` / `lastPaidCents` (newest checkout’s amount — used for “latest activity”).
3. Aggregate click events into `clicksLastHour`, `clicksDaily`, `clicksWeekly`, `clicksMonthly`, `clicksAllTime`.
4. Return the list. The frontend sorts by the selected range.

Sort rule for a range:

1. Higher paid total in that window first.
2. If equal, earlier `createdAt` first.
3. If still equal, lower product id first.

Cost-per-click on a row is `window paid / window clicks`.

**Trending** (frontend): `clicksLastHour`, not money.

**Latest activity** (frontend): listings that have `lastPaidAt`. The dollar amount shown is `lastPaidCents` (that checkout), not the window total.

The frontend query `?range=daily|weekly|monthly|all` only changes how the client sorts the same payload. Omit `range` for monthly.

---

## 5. Listing identity

Input is a website (`yoursite.com` or a full URL).

- Lowercased host, `www.` stripped, trailing slash stripped.
- `listing_key` = `host + path`.
- Same key = same listing. Paying again adds money; it does not create a second row.

At least one category, at most 15, from the fixed list in `internal/domain/category.go`.

---

## 6. Money rules

`POST /api/checkouts` body: `{ target, amountCents, categories }`.

`amountCents` **is the payment**.

| Situation | Allowed |
| --- | --- |
| New listing | `$0` or any whole dollar up to `$999,999` |
| Already listed | at least `$1`, whole dollars, same ceiling |

`$0` skips Dodo and fulfills immediately. Anything above that goes to hosted checkout (unless `PAYMENTS_MODE=simulate`, which fulfills immediately too).

After fulfill:

```
products.bid_cents += paid_cents
checkouts.status = paid, paid_at = now, product_id = listing
```

Two concurrent checkouts cannot both add from the same stale `bid_cents`: the update is compare-and-set on `bid_cents` + `last_checkout_id`. A webhook retry with the same checkout is a no-op.

---

## 7. Actions

### List the board

`GET /api/products` optional `?category=` and `?q=`.

Attaches windowed paid + click totals. Does not sort by range (the client does).

### Pay / list

```
frontend claim form
  → POST /api/checkouts
      validate URL, amount, categories
      insert pending checkout
      $0 or simulate → fulfill now, return product + rank
      else → Dodo session, return checkoutUrl
  → user pays on Dodo
  → POST /api/webhooks/dodo  (payment.succeeded)
       or GET /api/checkouts/{id} after redirect (poll if webhook is late)
  → fulfill: create product or add paid_cents
  → fetch the site for tagline + favicon
```

Fulfill for a **new** URL inserts `products`. Fulfill for an **existing** URL adds to `bid_cents` and may update categories / tagline / icon.

Checkout metadata carries `checkout_id`. That is how the webhook finds the pending row.

### Click through

`POST /api/products/{id}/clicks` with `{ visitorId }`.

1. Try to insert a click event with this hour’s `dedup_key`.
2. Duplicate key → not counted, current total returned.
3. Inserted → increment `products.clicks`, return new total.

Frontend also remembers clicks in `sessionStorage` so the UI does not double-fire in the same tab.

### Refresh tagline / icon

`POST /api/products/{id}/meta` fetches the live site (10s cooldown). Does **not** create activity; activity is payments only.

### Presence

`POST /api/presence` `{ visitorId, visit }`. Heartbeat. `visit: true` counts a page load.

`GET /api/stats` returns unique visitors, visits, and `since` (launch date).

---

## 8. HTTP API

| Method | Path | Why it exists |
| --- | --- | --- |
| `GET` | `/api/products` | The board. Optional `?category=` and `?q=`. |
| `GET` | `/api/products/{id}` | One listing + all-time rank. |
| `POST` | `/api/products/{id}/clicks` | Outbound click. |
| `POST` | `/api/products/{id}/meta` | Refresh tagline/icon from the site. |
| `POST` | `/api/checkouts` | Validate payment, start Dodo or fulfill. |
| `GET` | `/api/checkouts/{id}` | Status after redirect. Syncs from Dodo if the webhook is late. |
| `POST` | `/api/webhooks/dodo` | `payment.succeeded` fulfills; failed/cancelled marks failed. |
| `GET` | `/api/categories` | Fixed category list. |
| `GET` | `/api/health` | Process + payments config. |
| `GET` | `/api/stats` | Visitors / visits. |
| `POST` | `/api/presence` | Heartbeat. |

---

## 9. Run it

```bash
cp .env.example .env
docker compose up -d
go run ./cmd/api
```

`docker compose up -d` starts MongoDB on `localhost:27017`. Atlas works too: set `MONGODB_URI` to your `mongodb+srv://...` connection string.

`PAYMENTS_MODE=simulate` applies payments immediately so you can work on the UI without a Dodo account.

### Dodo Payments

1. Create a **single payment** product in the Dodo dashboard.
2. Turn on **Pay What You Want**.
3. Set the minimum to `$1` and the maximum to `$999,999`.
4. Put the product id and keys in `.env`:

```bash
PAYMENTS_MODE=dodo
DODO_ENVIRONMENT=test_mode
DODO_PAYMENTS_API_KEY=...
DODO_PAYMENTS_WEBHOOK_KEY=...
DODO_PRODUCT_ID=pdt_...
```

5. Point the Dodo webhook at `https://<your-api>/api/webhooks/dodo`.
6. For local webhooks, use a tunnel (ngrok, Cloudflare Tunnel, or Dodo's CLI) to that same path.

`DODO_ENVIRONMENT`, the API key, webhook secret, and product id must all be from the same mode (`test_mode` vs `live_mode`). Mixing them makes checkout fail.
