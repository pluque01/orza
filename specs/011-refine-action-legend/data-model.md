# Presentation Model: Refine Action Legend

This feature introduces no persisted entities or storage changes. It uses the following in-memory presentation concepts.

## Contextual Action Descriptor

Represents one existing action currently applicable to the selected browser object and focus.

| Field | Meaning | Rules |
|-------|---------|-------|
| Identifier | Stable action identity | Continues to map a key event to existing dispatch behavior. |
| Key display | Visible key or key combination | Must match the effective existing binding. |
| Action label | Short user-facing description | Is rendered as the muted value text. |
| Applicability | Current selection and focus eligibility | Controls Help and dispatch availability. |
| Category | Navigation, connection, management, or application | Determines Help group and stable group order. |

## Legend Entry

Represents a contextual action selected for the compact browser legend.

| Field | Meaning | Rules |
|-------|---------|-------|
| Source descriptor | Reference to a contextual action descriptor | Does not own a second key binding or applicability rule. |
| Display order | Position in the legend | Navigation; optional Connect; creation; Quit. |
| Key text | Emphasized key display | Remains recognizable when ANSI styling is removed. |
| Action text | Muted action description | Is semantically identical to the source descriptor label. |

**Selection rules**:

- Include Move Up, Move Down, New connection, New folder, and Quit when they are applicable.
- Include Connect only for a selected connectable connection.
- Exclude secondary actions from the legend without changing their Help or dispatch availability.

## Help Group

Represents an ordered collection of all currently applicable action descriptors.

| Field | Meaning | Rules |
|-------|---------|-------|
| Group name | User-visible action category | Order: navigation, connection, management, application. |
| Rows | Key/action pairs in the group | Every applicable descriptor occurs exactly once across all groups. |
| Key column | Key or combination field | Uses the same emphasis treatment as the legend key. |
| Action column | Action description field | Uses the same muted treatment as the legend description. |

## State Transitions

- Selection or focus changes -> recompute contextual descriptors -> derive the legend and Help rows -> render the next visible state.
- Connectable connection selected -> Connect included in legend and Help.
- Non-connectable selection, folder, or empty browser -> Connect absent from legend and Help.
- Help opened -> display grouped rows in a scrollable modal while retaining the close control.
