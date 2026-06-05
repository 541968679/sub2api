-- Keep banner-only dismissals from inheriting the legacy read_at default.
ALTER TABLE announcement_reads
    ALTER COLUMN read_at DROP DEFAULT;
