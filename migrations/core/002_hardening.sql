ALTER TABLE expense_items
    ADD CONSTRAINT expense_items_id_expense_unique UNIQUE (id, expense_id);

ALTER TABLE allocations
    ADD CONSTRAINT allocations_item_same_expense_fk
    FOREIGN KEY (expense_item_id, expense_id)
    REFERENCES expense_items (id, expense_id);
