"""Add cascade to list_webhook_subscription

Revision ID: d4b5b0b5c3f6
Revises: c09158fb4d2d
Create Date: 2022-07-20 06:16:21.264427

"""

# revision identifiers, used by Alembic.
revision = 'd4b5b0b5c3f6'
down_revision = 'c09158fb4d2d'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE list_webhook_subscription DROP CONSTRAINT list_webhook_subscription_list_id_fkey;
    ALTER TABLE list_webhook_subscription ADD CONSTRAINT list_webhook_subscription_list_id_fkey FOREIGN KEY (list_id) REFERENCES list(id) ON DELETE CASCADE;
    """)


def downgrade():
    op.execute("""
    ALTER TABLE list_webhook_subscription DROP CONSTRAINT list_webhook_subscription_list_id_fkey;
    ALTER TABLE list_webhook_subscription ADD CONSTRAINT list_webhook_subscription_list_id_fkey FOREIGN KEY (list_id) REFERENCES list(id);
    """)
