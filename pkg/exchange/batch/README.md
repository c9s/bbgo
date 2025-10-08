# Batch Query Module

## Basic Assumptions for `.Query()` Methods

All `.Query()` methods in the `batch` module follow some fundamental assumptions that are crucial for proper usage:

### Error Channel Guarantee

The **error channel is always guaranteed to be closed** by the Query method implementation. This means that after the data channel has been fully consumed, reading from the error channel will **never block** - it will either return an error or immediately signal that the channel is closed.

### Common Usage Pattern

Due to this guarantee, the typical pattern for using `.Query()` methods is:

1. **First**: Read all data from the data channel
2. **Then**: Read from the error channel to handle any errors

This pattern ensures that:
- You process all available data before checking for errors
- Your code won't hang waiting for error channel reads
- Error handling is performed after data consumption is complete

### Example Usage

```go
// Query returns (dataChan, errorChan)
dataChan, errChan := batchQuery.Query(ctx, symbol, options)

// Read all data first
var results []types.Trade
for trade := range dataChan {
    results = append(results, trade)
}

// Then check for errors - this will not block
if err := <-errChan; err != nil {
    log.WithError(err).Error("batch query failed")
    return err
}

// Process `results`...
```
or 
```go
// in a function
for {
    select {
    case <-ctx.Done():
        return nil
    case data, ok := <-dataC:
        if !ok {
            err := <-errChan
            return err
        }
        // do stuff with `data`...
    }
}
```

This design pattern provides a clean separation between data processing and error handling while ensuring predictable, non-blocking behavior.
