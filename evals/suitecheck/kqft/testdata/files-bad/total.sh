set -eu
sum=1
for value do sum=$((sum + value)); done
printf '%s\n' "$sum"
