# saaslb-backend

Go API for the monthly pay-to-rank leaderboard. Listings, clicks, and bids live here. Money is collected through [Dodo Payments](https://docs.dodopayments.com) hosted checkout.

## What the frontend calls

| Method | Path | Why it exists |
| --- | --- | --- |
| `GET` | `/api/products` | The board. Optional `?category=` and `?q=`. |
| `POST` | `/api/products/{id}/clicks` | Outbound click counter. |
| `POST` | `/api/checkouts` | Validate a bid, then start Dodo checkout (or apply it in `simulate` mode). |
| `GET` | `/api/checkouts/{id}` | Status after Dodo redirects back. Syncs from Dodo if the webhook is late. |
| `POST` | `/api/webhooks/dodo` | Dodo `payment.succeeded` applies the bid. |
| `GET` | `/api/categories`, `/api/period`, `/api/health` | Shared lists and month rollover. |
| `GET` | `/api/stats` | Current online count and all-time visits. |
| `POST` | `/api/presence` | Heartbeat. `visit: true` counts one page load. |

A bid is the **target rank amount**. For a new listing you pay that amount. For a raise you only pay the difference.

When the calendar month changes, every listing drops back to `$0`. Same listing first still sits higher.

## Run it

```bash
cp .env.example .env
docker compose up -d
go run ./cmd/api
```

`docker compose up -d` starts MongoDB on `localhost:27017`. Atlas works too: set `MONGODB_URI` to your `mongodb+srv://...` connection string.

The API listens on `http://localhost:8080`. Point the frontend at it with the Vite `/api` proxy, or set `VITE_API_URL`.

`PAYMENTS_MODE=simulate` applies bids immediately so you can work on the UI without a Dodo account.

## Dodo Payments

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

Checkout metadata carries `checkout_id`. The webhook (or the return-URL status poll) uses that id to apply the bid exactly once.
# saaslb-backend
