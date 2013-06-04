
rm -f whereismyfox
GOPATH=`pwd` go build whereismyfox.com/whereismyfox

pushd app
zip -u package.zip \
    index.html \
    manifest.webapp \
    logos/64.png \
    logos/128.png
popd
