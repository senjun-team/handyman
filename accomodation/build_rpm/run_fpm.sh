set -e 

fpm \
  -s dir -t rpm \
  -p handyman-0.1.0-1-any.rpm \
  --name handyman \
  --license agpl3 \
  --version 0.1.0 \
  --architecture all \
  --description "Go!" \
  handyman/main=/bin/handyman ../accomodation/handyman.service=/etc/systemd/system/handyman.service
