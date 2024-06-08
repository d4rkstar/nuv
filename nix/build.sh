#!/usr/bin/env nix-shell
#! nix-shell -i bash -p nix-prefetch-git -p jq -p nix -p go

# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

# Default values
# Define the repository information
OWNER="nuvolaris"
REPO="nuv"
REV=""
CACHIX=""
BUILD_DIR=$(mktemp -d)
INIT_DIR=$(pwd)

cleanup() {
  echo "Removing $BUILD_DIR"
  rm -f "$BUILD_DIR/$REPO.nix" "$BUILD_DIR/default.nix"
  rm -rf "$BUILD_DIR"
}

# Function to print help
print_help() {
    echo "Usage: $0 [-v VERSION] [-c CACHIX] [-h]"
    echo ""
    echo "Options:"
    echo "  -v VERSION   Build package at version VERSION (check https://github.com/$OWNER/$REPO/releases)"
    echo "  -c CACHIX    Use cachix with cache named <CACHIX>"
    echo "  -h           Print this help message and exit"
}

# Parse command-line options
while getopts ":v:c:h" opt; do
    case ${opt} in
        v )
            REV=$OPTARG
            ;;
        c )
            CACHIX=$OPTARG
            ;;
        h )
            print_help
            exit 0
            ;;
        \? )
            echo "Invalid option: -$OPTARG" 1>&2
            print_help
            exit 1
            ;;
        : )
            echo "Option -$OPTARG requires an argument." 1>&2
            print_help
            exit 1
            ;;
    esac
done
shift $((OPTIND -1))

if [ -z "$REV" ]; then
    echo "Error: -v option is required." 1>&2
    print_help
    exit 1
fi

if [ ! -z "$CACHIX" ] && [ -z "$CACHIX_AUTH_TOKEN" ]; then
  echo "Use cachix require CACHIX_AUTH_TOKEN set in env"
  exit 1
fi

echo "Moving to $BUILD_DIR"
cd $BUILD_DIR
echo "PWD IS $(pwd)"

echo -n "Calculating hash for $REPO at version $REV: "
# Fetch the repository and get the hash
JSON=$(nix-prefetch-git --quiet https://github.com/$OWNER/$REPO.git $REV)

# Extract the sha256 hash from the JSON output
SHA256=$(echo $JSON | jq -r .hash)
echo "$SHA256"
echo -n "Calculating hash for go mod vendor in $REPO at version $REV"
REPO_DIR=$(mktemp -d)
git clone --quiet https://github.com/$OWNER/$REPO.git $REPO_DIR
cd $REPO_DIR
git checkout --quiet $REV

# Ensure go modules are used
export GO111MODULE=on

# Create the vendor directory
go mod vendor

# Calculate the vendor hash
VENDOR_HASH=$(nix hash path ./vendor)
echo "Calculated hash for go mod vendor in $REPO at version $REV: $VENDOR_HASH"

echo "Cleanup and build"
cd ..
rm -rf $REPO_DIR

cd $BUILD_DIR

read -r -d '' NIX_DEFAULT << EOM
{ pkgs ? import <nixpkgs> {}, 
  version ? ""
}:
let  
  nuv = pkgs.callPackage ./nuv.nix {}; 
in
pkgs.mkShell {
  buildInputs = [
    nuv
  ];
}
EOM


# Generate the Nix expression template
read -r -d '' NIX_EXPRESSION << EOM
{ lib
, stdenv
, pkgs
, fetchFromGitHub
, fetchurl
, buildGoModule
, makeWrapper
, breakpointHook
, jq
, curl
, kubectl 
, eksctl 
, kind 
, k3sup 
, coreutils
}:

let 
   branch = "3.0.0";
   version = "$REV";
in buildGoModule rec {

  pname = "nuv";

  inherit branch version;

  nativeBuildInputs = [ makeWrapper jq curl breakpointHook ] ;

  buildInputs = [ kubectl eksctl kind k3sup coreutils ];

  subPackages = ["."];

  src = fetchFromGitHub {
    owner = "$OWNER";
    repo = "$REPO";
    rev = "$REV";
    sha256 = "$SHA256";    
  };
  
  vendorHash = "$VENDOR_HASH";

  doCheck = false; 

  ldflags =  [
    "-X main.NuvVersion=${version}"
    "-X main.NuvBranch=${branch}"
  ];

  meta = with lib; {
    description = "Nuvolaris Almighty CLI tool";
    license = licenses.asl20;
    homepage = "https://nuvolaris.io/";
    maintainers = with maintainers; [ msciabarra d4rkstar ];
    mainProgram = "nuv";
  };

  postInstall  = ''
    makeWrapper \${coreutils}/bin/coreutils \$out/bin/coreutils
    makeWrapper \${kubectl}/bin/kubectl \$out/bin/kubectl
    makeWrapper \${eksctl}/bin/eksctl \$out/bin/eksctl
    makeWrapper \${kind}/bin/kind \$out/bin/kind
    makeWrapper \${k3sup}/bin/k3sup \$out/bin/k3sup
  '';
}
EOM

# Write the Nix expression to a file
rm -f nuv.nix default.nix
echo "$NIX_DEFAULT" > default.nix
echo "$NIX_EXPRESSION" > nuv.nix

# Build the project using the new Nix expression
STORE=$(nix-build --no-out-link default.nix)
echo "Built $STORE"

if [ ! -z $CACHIX ]; then
  echo $STORE | cachix push $CACHIX
fi

# Final cleanup
cd $INIT_DIR
cleanup