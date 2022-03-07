## Dnum: High Precision Numeric Implementation
----------------------------------------------
The `dnum` version of `fixedpoint` supports up to 16 digits of decimal precision. It's two times slower than the legacy version, which only supports up to 8 digits of decimal precision. We recommend that strategy developers do algorithmic calculations in `float64`, then convert them back to `fixedpoint` to interact with exchanges to keep the balance between speed and the accuracy of accounting result.

To Install dnum version of bbgo, we've create several scripts for quick setup:

```sh
# grid trading strategy for binance exchange
bash <(curl -s https://raw.githubusercontent.com/c9s/bbgo/main/scripts/setup-grid-dnum.sh) binance

# grid trading strategy for max exchange
bash <(curl -s https://raw.githubusercontent.com/c9s/bbgo/main/scripts/setup-grid-dnum.sh) max

# bollinger grid trading strategy for binance exchange
bash <(curl -s https://raw.githubusercontent.com/c9s/bbgo/main/scripts/setup-bollgrid-dnum.sh) binance

# bollinger grid trading strategy for max exchange
bash <(curl -s https://raw.githubusercontent.com/c9s/bbgo/main/scripts/setup-bollgrid-dnum.sh) max
```

If you already have the configuration somewhere, you may want to use the download-only script:
```sh
bash <(curl -s https://raw.githubusercontent.com/c9s/bbgo/main/scripts/download-dnum.sh)
```

The precompiled dnum binaries are also available in the [Release Page](https://github.com/c9s/bbgo/releases).
