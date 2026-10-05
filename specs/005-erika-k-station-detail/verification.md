# Erika-K station detail verification

The golden fixtures were generated from the pre-refactor implementation and
committed before the implementation changes.

```text
go -C apps/server test ./internal/hostprogram/erikak -run 'TestScreensAndBoardsMatchGolden' -count=3
go -C apps/server test ./...
go -C apps/server vet ./...
```

The golden test compares all 29 boards and the complete scripted terminal
transcript byte-for-byte. The preset supplies Erika-K board and text detail;
the runtime has no station-specific board tree or station wording.
