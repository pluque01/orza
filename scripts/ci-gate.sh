#!/usr/bin/env bash
set -euo pipefail

required='format test race vet static vulnerability build vendor secret'
seen=' '
failed=0

for result in "$@"; do
  case $result in
    *=*) category=${result%%=*}; status=${result#*=} ;;
    *) printf 'CI aggregate: malformed result <%s>\n' "$result" >&2; exit 2 ;;
  esac

  case " $required " in
    *" $category "*) ;;
    *) printf 'CI aggregate: unknown check <%s>\n' "$category" >&2; exit 2 ;;
  esac
  case $seen in
    *" $category "*) printf 'CI aggregate: duplicate check <%s>\n' "$category" >&2; exit 2 ;;
  esac
  seen="$seen$category "

  case $status in
    success | passed) ;;
    excepted)
      if [ "$category" != vulnerability ]; then
        printf 'CI aggregate: %s cannot use a vulnerability exception\n' "$category" >&2
        failed=1
      fi
      ;;
    *)
      printf 'CI aggregate: %s did not pass (status=%s)\n' "$category" "$status" >&2
      failed=1
      ;;
  esac
done

for category in $required; do
  case $seen in
    *" $category "*) ;;
    *) printf 'CI aggregate: required check %s is missing\n' "$category" >&2; failed=1 ;;
  esac
done

[ "$failed" -eq 0 ] || exit 1
printf 'CI aggregate: every required check passed\n'
