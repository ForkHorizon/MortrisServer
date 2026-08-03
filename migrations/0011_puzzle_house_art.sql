-- Assembled house art, so the diagram can sit over a picture of the real
-- house instead of floating on an empty canvas
-- (docs/puzzle-visual-analytics-plan.md, "Art tier: both layers").
--
-- Stored in Postgres rather than on disk. Measured: the 103 houses come to
-- about 4 MB as cropped WebP, which is small enough that a filesystem
-- store would buy nothing and cost a new config path, a new backup
-- concern, and orphan cleanup. In here it is transactional with the
-- revision it belongs to and already covered by pgbackrest.
--
-- No placement columns: the cropped art's extent is exactly the union of
-- the revision's block bounds, verified by overlaying outlines on the art.
-- Storing the rect would duplicate state the renderer already computes for
-- its viewBox, and the two could then disagree.
CREATE TABLE IF NOT EXISTS puzzle_house_art (
    project_id       text NOT NULL,
    content_revision text NOT NULL,
    city_id          integer NOT NULL,
    house_id         integer NOT NULL,
    media_type       text NOT NULL,
    width            integer NOT NULL,
    height           integer NOT NULL,
    image            bytea NOT NULL,
    imported_at      timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (project_id, content_revision, city_id, house_id),
    FOREIGN KEY (project_id, content_revision)
        REFERENCES puzzle_content_revisions(project_id, content_revision) ON DELETE CASCADE
);
