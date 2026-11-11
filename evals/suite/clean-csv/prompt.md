---
name: clean-csv
runs: 1
max_turns: 20
allowed_tools: [Read, Write, Bash]
---

Remove duplicate data rows from inputs/items.csv, preserve the header, sort data rows, and save clean.csv. Write summary.txt containing Unique items: 2.
