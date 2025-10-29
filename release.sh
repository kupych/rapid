#!/bin/bash
# release.sh - Automate AUR stable package updates

if [ -z "$1" ]; then
    echo "Usage: ./release.sh <version>"
    echo "Example: ./release.sh 0.0.5"
    exit 1
fi

VERSION=$1

echo "📦 Releasing v$VERSION"

# Update main repo
echo "🏷️  Tagging GitHub release..."
git tag "v$VERSION"
git push origin "v$VERSION"
git push

# Wait for GitHub to generate tarball
echo "⏳ Waiting 5s for GitHub to generate release tarball..."
sleep 5

# Update AUR stable package
echo "📝 Updating AUR package..."
cd ~/aur/rapid || exit

# Update version in PKGBUILD
sed -i "s/^pkgver=.*/pkgver=$VERSION/" PKGBUILD
sed -i "s/^pkgrel=.*/pkgrel=1/" PKGBUILD

# Get new checksum
echo "🔐 Generating checksum..."
CHECKSUM=$(makepkg -g 2>&1 | grep sha256sums | cut -d"'" -f2)
sed -i "s/^sha256sums=.*/sha256sums=('$CHECKSUM')/" PKGBUILD

# Regenerate .SRCINFO
makepkg --printsrcinfo > .SRCINFO

# Commit and push
git add PKGBUILD .SRCINFO
git commit -m "Update to v$VERSION"
git push

echo "✅ Released v$VERSION to GitHub and AUR!"
echo "📋 Don't forget to update ROADMAP.md"
