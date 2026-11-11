set -eu
sum=0
for value do sum=$((sum + value)); done
printf '%s\n' "$sum"
