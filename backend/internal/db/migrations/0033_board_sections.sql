-- Swimlanes are called sections. The index and foreign key names (ix_bs_board,
-- fk_bs_board) already fit board_sections.
ALTER TABLE boards RENAME COLUMN swimlanes TO section_by;
RENAME TABLE board_swimlanes TO board_sections;
