# Requirements: Plan Table Notes Sub-Row

## Scope

Move the "Notes" column from the plan table on the strategy page to a second/grouped row beneath each plan item. This fixes horizontal overflow on mobile and improves desktop readability.

**Does:** Remove Notes column, add notes as a sub-row with colspan=5.
**Does NOT:** Change plan data structure, modify JS logic, touch any other page.

## File Changes

### 1. `pages/strategy.html` (lines 58-88)

**Remove:**
- `<th>Notes</th>` header (line 71)
- `<td x-text="item.notes || '-'" class="plan-notes-cell">` column (line 82)

**Replace** the `<template x-for>` block (lines 74-84) with:

```html
<template x-for="item in planItems" :key="item.id">
    <tbody>
        <tr :class="item.status === 'completed' ? 'plan-completed' : ''"
            class="plan-item-row"
            :style="item.notes ? 'border-bottom:none' : ''">
            <td><span class="plan-status" :class="'plan-status-' + item.status" x-text="item.status"></span></td>
            <td><span class="plan-action" :class="'plan-action-' + (item.action || '').toLowerCase()" x-text="item.action || '-'"></span></td>
            <td x-text="item.ticker || '-'" style="white-space:nowrap;"></td>
            <td x-text="item.description"></td>
            <td x-text="item.deadline ? new Date(item.deadline).toLocaleDateString('en-AU', {day:'numeric',month:'short',year:'numeric'}) : '-'" style="white-space:nowrap;"></td>
        </tr>
        <tr x-show="item.notes"
            :class="item.status === 'completed' ? 'plan-completed' : ''"
            class="plan-notes-row">
            <td colspan="5" class="plan-notes-cell" x-text="item.notes"></td>
        </tr>
    </tbody>
</template>
```

**Pattern reference:** Dashboard holdings table (`pages/dashboard.html` lines 181-203) uses the same `<tbody>`-per-item pattern with `x-for`.

### 2. `pages/static/css/portal.css` (lines 1405-1409)

**Replace** existing `.plan-notes-cell` block with:

```css
/* Plan notes sub-row */
.plan-notes-row td {
    padding-top: 0;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid #ddd;
}
.plan-notes-cell {
    font-size: 0.8rem;
    color: #555;
    font-style: italic;
}
```

### 3. No JavaScript changes

`portfolioStrategy()` in `pages/static/common.js` unchanged. `planItems` array structure is the same; template reads `item.notes` in a new location.

## Test Cases

### Unit Tests
None needed — no Go code changes.

### UI Tests (`tests/ui/strategy_test.go`)

**Existing test `TestStrategyPlanEditor`** — should still pass (checks `.plan-table` visibility and PLAN header only).

**New test: `TestStrategyPlanNotesRow`**
- Navigate to `/strategy`
- Verify `.plan-table thead th` count is 5 (not 6)
- Verify no `<th>` contains text "Notes"
- Verify `.plan-notes-row` elements exist for items that have notes
- Verify `.plan-notes-cell` displays note text

## Edge Cases

1. **Empty notes**: `x-show="item.notes"` hides the notes row entirely
2. **Long notes**: Wraps naturally across full table width (no 300px constraint)
3. **Completed items**: `plan-completed` opacity applied to both rows via same `:class` binding
4. **Multiple `<tbody>`**: Valid HTML5, already used in dashboard holdings table
