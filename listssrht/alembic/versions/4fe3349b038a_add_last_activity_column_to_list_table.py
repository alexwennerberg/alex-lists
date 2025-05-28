"""Add last_activity column to list table.

Revision ID: 4fe3349b038a
Revises: 6197b8a2e537
Create Date: 2025-05-15 14:21:16.054508

"""

# revision identifiers, used by Alembic.
revision = '4fe3349b038a'
down_revision = '6197b8a2e537'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE list
    ADD COLUMN last_activity timestamp without time zone;
    """)


def downgrade():
    op.execute("""
    ALTER TABLE list DROP COLUMN last_activity;
    """)
