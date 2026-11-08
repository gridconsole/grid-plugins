// Pin only the wall clock. Filesystem clocks and random IDs are normalized after capture.
const NativeDate = Date;
const instant = NativeDate.parse('2025-01-02T03:04:05.000Z');
globalThis.Date = class extends NativeDate {
  constructor(...args) { super(...(args.length ? args : [instant])); }
  static now() { return instant; }
};
