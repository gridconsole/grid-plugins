schtasks /Create /TN {{LABEL}} /XML {{HOME}}/.grid-routine-timers/{{LABEL}}.xml
schtasks /Delete /TN {{LABEL}} /F
