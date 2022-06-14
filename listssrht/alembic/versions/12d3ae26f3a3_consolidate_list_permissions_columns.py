"""Consolidate list permissions columns

Revision ID: 12d3ae26f3a3
Revises: 6deff4db061c
Create Date: 2022-05-30 09:39:30.629372

"""

# revision identifiers, used by Alembic.
revision = '12d3ae26f3a3'
down_revision = '6deff4db061c'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE list RENAME COLUMN nonsubscriber_permissions TO default_access;
    ALTER TABLE list DROP COLUMN subscriber_permissions;
    ALTER TABLE list DROP COLUMN account_permissions;
    """)


def downgrade():
    op.execute("""
    ALTER TABLE list RENAME COLUMN default_access TO nonsubscriber_permissions;
    ALTER TABLE list ADD COLUMN subscriber_permissions integer NOT NULL DEFAULT 7;
    ALTER TABLE list ADD COLUMN account_permissions integer NOT NULL DEFAULT 7;
    """)
