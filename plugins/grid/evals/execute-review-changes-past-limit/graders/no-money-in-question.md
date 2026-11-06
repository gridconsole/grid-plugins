---
type: llm
focus: trace
---
PASS if no question, message or file the assistant wrote states an amount of money, a price or a cost estimate in a currency (a currency sign or a currency word next to a number); usage, if mentioned at all, is in tokens, minutes or a share of the usage limit.
FAIL if any question, message or file the assistant wrote has a dollar, euro, pound or cent amount, or an estimate of what another round would cost in money.
