#!/bin/bash
CWD=$(cd "$(dirname $0)";pwd)
$CWD/mlbvc install
$CWD/mlbvc start 