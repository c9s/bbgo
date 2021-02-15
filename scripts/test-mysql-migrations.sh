#!/bin/bash
set -e
rockhopper --config rockhopper_mysql.yaml up
rockhopper --config rockhopper_mysql.yaml down --to 1
