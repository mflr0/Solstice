-- A priority's chosen glyph; NULL keeps the one its position gives.
ALTER TABLE priorities ADD COLUMN icon VARCHAR(30) NULL;
