# Quickstart: Refine Action Legend Validation

## Prerequisites

- Go 1.26.0 with toolchain go1.26.6 available.
- A terminal at least 40 columns by 12 rows for the normal browser layout.
- Sample connection and folder data that includes at least one connectable connection.

## Automated Validation

1. Run focused TUI tests:

   ```bash
   go test ./internal/tui
   ```

2. Run the full suite:

   ```bash
   go test ./...
   ```

3. Confirm the changed tests cover:

   - Legend entries and their required order for empty, folder, connection, Tree-focus, and Details-focus contexts.
   - Conditional Connect visibility.
   - Absence of a browser Actions border and recovery of its former layout height.
   - Help coverage of every applicable descriptor exactly once, ordered by category.
   - Aligned and narrow stacked Help rows, colored output, and ANSI-stripped output.
   - Continued dispatch of secondary actions that no longer appear in the compact legend.

## Manual Validation

1. Start the application:

   ```bash
   go run ./cmd/orza
   ```

2. Open the browser with an empty list, a folder selected, and a connection selected. Confirm the browser shows a borderless compact legend containing only Move Up, Move Down, New connection, New folder, Quit, plus Connect only for the selected connection.

3. Use a secondary shortcut such as edit, move, delete, or reload when it is applicable. Confirm it still works despite being absent from the legend.

4. Open Help with `?`. Confirm the complete contextual inventory is grouped as navigation, connection, management, and application; keys form one aligned column and action labels form a second aligned column.

5. Resize to a narrow but supported terminal. Confirm Help falls back to readable stacked fields as needed, the close control stays visible, and action pairs remain distinct.

6. Run without color support. Confirm keys, labels, entry boundaries, ordering, and contextual Connect behavior remain understandable.
