# `Run` Command

There are three modes:
1. `runSetup`
2. `runConfig`: the main flow, executes using the given user config (YAML)
3. `runWrapperBinary`

## `runConfig` Flow
1. Create a `bbgo.Environment` object
2. Bootstrap environment
   - Configure Database (`bbgo.Environment.ConfigureDatabase`)
   - Configure Exchange Sessions (`bbgo.Environment.ConfigureExchangeSessions`)
     - Exchange objects are generated and corresponding `bbgo.ExchangeSession` objects are created in this step
     - Exchange session objects will be initialized in subsequent processes
3. Environment Init (`bbgo.Environment.Init`)
   - Exchange session initialization (`bbgo.ExchangeSession.Init`)
4. (optional) Environment Sync
5. Execute Trader
   - Generate Trader objects
   - Initialize
   - Load state
   - Run
6. Wait for signal to stop process
7. Graceful Shutdown