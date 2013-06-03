
rm -f whereismyfox
GOPATH=`pwd` go build whereismyfox.com/whereismyfox

pushd static
zip -u package.zip \
    index.html \
    manifest.webapp \
    push.html \
    style.css \
    logos/64.png \
    logos/128.png
popd
