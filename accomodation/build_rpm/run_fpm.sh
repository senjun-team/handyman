set -e 

version="0.1.20"

fpm \
  -s dir -t rpm \
  -p handyman-$version-1-any.rpm \
  --name handyman \
  --license agpl3 \
  --version $version \
  --architecture all \
  --description "Middleware between user & watchman" \
  handyman/main=/bin/handyman ../accomodation/handyman.service=/etc/systemd/system/handyman.service
