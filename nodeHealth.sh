#!/bin/sh

######################
# Author: Abhishek
# Date: 01/12/2022
#
# This script outputs node health
#
# Version: v1
#####################

set -x # debug mod
set -e #exit the script when there is error
set -o pipefail

adada | echo "as"

df -h


free -g


nproc

