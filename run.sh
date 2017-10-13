#!/bin/bash

docker build  -t secret-santa  .

docker run -v $PWD/output:/santa/output/ secret-santa $@
