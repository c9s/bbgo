#!/bin/bash
set -e
rockhopper --config rockhopper_sqlite.yaml create --type sql $1
rockhopper --config rockhopper_mysql.yaml create --type sql $1
