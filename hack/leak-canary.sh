#!/usr/bin/env bash
# This repository is public and its history cannot be unpublished — a
# rewrite changes the SHAs but not what was already fetched. So the rule
# ("mechanism only; particulars are caller inputs") is enforced
# mechanically rather than remembered.
#
# The rule has teeth here because of what these tools DO: they take an
# account id, a region, a secret id and a profile and write to your home
# directory. Every one of those is a caller argument — `--ecr`,
# `--secret-id`, `--profile`, `--region`. That is the whole reason this
# repository can be public, and this script is what keeps it true.
#
# Copied from truvity/ci-workflows, which is public for the same reason.
# Add a pattern here the first time something new turns out to be a
# particular. Never add an exception without one.
set -uo pipefail

patterns=(
  '[0-9]{12}'                          # AWS account id
  'arn:aws'                            # any ARN
  '[0-9]{12}\.dkr\.ecr\.'              # ECR registry host
  '\.svc\.cluster\.local'              # in-cluster DNS
  '/secrets/'                          # SSM parameter paths
  'truvity-[a-z0-9-]*-(ci-cache|artifacts|state)'   # S3 buckets
  '\.truvity\.(xyz|com|co)'            # internal hostnames
  'glpat-|ghp_|github_pat_'            # tokens, in case of an accident
)

fail=0
for p in "${patterns[@]}"; do
  # Exclude this script: it necessarily contains the patterns it bans.
  if hits=$(grep -rInE "$p" . \
              --exclude-dir=.git \
              --exclude="leak-canary.sh" 2>/dev/null); then
    echo "LEAK: pattern /$p/ matched — particulars belong in caller arguments:"
    echo "$hits" | head -5 | sed 's/^/    /'
    fail=1
  fi
done

if [ "$fail" = 0 ]; then
  echo "leak canary clean — ${#patterns[@]} patterns checked, no particulars found"
fi
exit $fail
