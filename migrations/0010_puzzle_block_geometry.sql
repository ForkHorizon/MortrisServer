-- Block geometry for the visual house view (docs/puzzle-visual-analytics-plan.md).
-- Until now a block was a single centre point, which is not enough to draw a
-- house: the dashboard can plot 56 dots but cannot show which detail sits
-- where, or paint one by its fall rate.
--
-- Bounds are four plain integer columns rather than one jsonb blob because
-- the renderer's viewBox is a MIN/MAX aggregate over a house's blocks —
-- cheap in SQL over columns, awkward over jsonb extraction. The outline is
-- only ever shipped whole, so jsonb is the right shape for it.
--
-- All five columns are nullable: revisions imported before this migration
-- have no geometry and must keep resolving. Events are always interpreted
-- against their own revision, so an old attempt simply renders without
-- shapes rather than borrowing newer ones.
ALTER TABLE puzzle_content_blocks
    ADD COLUMN IF NOT EXISTS bounds_min_x_milli integer,
    ADD COLUMN IF NOT EXISTS bounds_min_y_milli integer,
    ADD COLUMN IF NOT EXISTS bounds_max_x_milli integer,
    ADD COLUMN IF NOT EXISTS bounds_max_y_milli integer,
    ADD COLUMN IF NOT EXISTS outline_milli jsonb;
