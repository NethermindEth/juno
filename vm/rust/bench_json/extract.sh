#!/usr/bin/env bash

for i in *.out
do 
  echo -n "$i,"
  cat $i | jq '.mean.estimate | select(. != null)' | awk '{printf "%s,", $0; getline; print}'
done 
