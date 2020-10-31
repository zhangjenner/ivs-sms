#!/bin/bash
CWD=$(cd "$(dirname $0)";pwd)
$CWD/mlbvc stop
$CWD/mlbvc uninstall 