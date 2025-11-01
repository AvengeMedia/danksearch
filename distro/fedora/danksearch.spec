# Spec for DankSearch - pre-built binary from GitHub releases

%global debug_package %{nil}
%global pkg_summary Blazingly fast and efficient file system search tool

Name:           danksearch
Version:        0.0.5
Release:        1%{?dist}
Summary:        %{pkg_summary}

License:        MIT
URL:            https://danklinux.com/docs/danksearch/
VCS:            https://github.com/AvengeMedia/danksearch

BuildRequires:  wget
BuildRequires:  gzip
BuildRequires:  coreutils
BuildRequires:  systemd-rpm-macros

Requires:       glibc

%description
DankSearch is a file system search utility designed for the Dank Linux modern
desktop suite. It provides rapid filesystem searching capabilities optimized
for performance and efficiency. The tool integrates seamlessly with
DankMaterialShell and its launcher system, enabling users to quickly locate
files across their system.

Powered by the bleve search library, DankSearch supports fuzzy search, EXIF
extraction, virtual folders, and concurrent indexing for blazingly fast results.

%prep
# Download and extract DankSearch binary for target architecture
case "%{_arch}" in
  x86_64)
    DSEARCH_ARCH="amd64"
    ;;
  aarch64)
    DSEARCH_ARCH="arm64"
    ;;
  *)
    echo "Unsupported architecture: %{_arch}"
    exit 1
    ;;
esac

# Pinned to v0.0.5 temporarily - v0.0.6 missing AMD64 builds due to GitHub auth issue
# TODO: Switch back to /latest/ once AMD64 builds are available in newer release
wget -O %{_builddir}/dsearch.gz "https://github.com/AvengeMedia/danksearch/releases/download/v0.0.5/dsearch-linux-${DSEARCH_ARCH}.gz" || {
  echo "Failed to download dsearch for architecture %{_arch}"
  exit 1
}
gunzip -c %{_builddir}/dsearch.gz > %{_builddir}/dsearch
chmod +x %{_builddir}/dsearch

# Download systemd user service file from repository
wget -O %{_builddir}/dsearch.service "https://raw.githubusercontent.com/AvengeMedia/danksearch/master/assets/dsearch.service" || {
  echo "Failed to download systemd service file"
  exit 1
}

%build
# Using pre-built binary - nothing to build

%install
# Install dsearch binary
install -Dm755 %{_builddir}/dsearch %{buildroot}%{_bindir}/dsearch

# Install systemd user service
install -Dm644 %{_builddir}/dsearch.service %{buildroot}%{_userunitdir}/dsearch.service

%files
%{_bindir}/dsearch
%{_userunitdir}/dsearch.service

%changelog
* Fri Oct 31 2025 DankLinux Team <noreply@danklinux.com> - 0.0.5-1
- Initial RPM package for DankSearch
- Pre-built binary from GitHub releases (pinned to v0.0.5)
- Includes systemd user service for autostart
- Note: v0.0.6 skipped due to missing AMD64 builds
