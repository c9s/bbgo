#!/bin/bash
rm -v bbgo.sqlite3 && rockhopper --config rockhopper_sqlite.yaml up && rockhopper --config rockhopper_sqlite.yaml down --to 1
