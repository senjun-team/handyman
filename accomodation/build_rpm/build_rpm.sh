#!/bin/bash

set -e

echo Start

cp -Rp /mnt/handyman .
cd handyman/cmd/handyman
go build main.go
cd ..

sh ../accomodation/build_rpm/run_fpm.sh

cp *.rpm /mnt/handyman/accomodation/build_rpm
echo RPM is ready
