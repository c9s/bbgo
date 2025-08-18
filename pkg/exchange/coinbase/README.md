# Order Life Cycle

- `market`/`limit` orders
  - Once `received` and the order is not **immediately** filled, it will becom `open`. Or it will go straight to `done`.
  - Optionally, there might be `change`s to the order.
  - If a `match` event is generated and the order is still fillable, a new `open` event will be generated
  - `done` is the terminate state of the order life cycle
```
                                           
 Received ───► Open ───► [Change* ───► Open]* ───► Done 
    │                                               ▲   
    └───────────────────────────────────────────────┘   
                                           
```
- `stop` order
  - After being created, it will generate an `activate` event.
  - [**Need to confirm**] After that, it will become a `limit` order and go through the life cycle depicted above.