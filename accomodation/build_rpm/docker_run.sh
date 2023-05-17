#!/bin/bash

docker run -it  \
  -v $(pwd)/../..:/mnt/handyman \
  handyman_rpm
