ALTER TABLE sites ADD COLUMN management TEXT NOT NULL DEFAULT 'managed' CHECK(management IN ('managed','external'));
ALTER TABLE sites ADD COLUMN lifecycle TEXT NOT NULL DEFAULT 'present' CHECK(lifecycle IN ('present','deleting','deleted'));
