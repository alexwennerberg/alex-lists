"""Add missing cascade to subscription_request

Revision ID: f7a8c3e239ce
Revises: f41a35421af6
Create Date: 2025-02-19 10:33:14.187310

"""

# revision identifiers, used by Alembic.
revision = 'f7a8c3e239ce'
down_revision = 'f41a35421af6'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE subscription_request
    DROP CONSTRAINT subscription_request_list_id_fkey,
    ADD CONSTRAINT subscription_request_list_id_fkey
    FOREIGN KEY (list_id) REFERENCES "list"(id) ON DELETE CASCADE;
    """)


def downgrade():
    op.execute("""
    ALTER TABLE subscription_request
    DROP CONSTRAINT subscription_request_list_id_fkey,
    ADD CONSTRAINT subscription_request_list_id_fkey
    FOREIGN KEY (list_id) REFERENCES "list"(id);
    """)
