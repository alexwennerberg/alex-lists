"""Add list visibility column

Revision ID: c09158fb4d2d
Revises: 12d3ae26f3a3
Create Date: 2022-05-30 10:44:19.524347

"""

# revision identifiers, used by Alembic.
revision = 'c09158fb4d2d'
down_revision = '12d3ae26f3a3'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    CREATE TYPE visibility AS ENUM (
        'PUBLIC', 'UNLISTED', 'PRIVATE'
    );

    ALTER TABLE list
    ADD COLUMN visibility visibility;

    UPDATE list
    SET visibility =
        CASE WHEN default_access & 1 > 0
            THEN 'PUBLIC'::visibility
            ELSE 'PRIVATE'::visibility
        END;

    ALTER TABLE list
    ALTER COLUMN visibility
    SET NOT NULL;
    """)


def downgrade():
    op.execute("""
    ALTER TABLE list DROP COLUMN visibility;
    DROP TYPE visibility;
    """)
