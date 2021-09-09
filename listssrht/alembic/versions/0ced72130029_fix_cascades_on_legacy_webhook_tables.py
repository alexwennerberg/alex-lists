"""Fix cascades on legacy webhook tables

Revision ID: 0ced72130029
Revises: 05c33e92b0e1
Create Date: 2021-09-09 10:35:55.513241

"""

# revision identifiers, used by Alembic.
revision = '0ced72130029'
down_revision = '05c33e92b0e1'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE list_webhook_subscription
    DROP CONSTRAINT list_webhook_subscription_list_id_fkey;

    ALTER TABLE list_webhook_subscription
    ALTER COLUMN list_id DROP NOT NULL;

    ALTER TABLE list_webhook_subscription
    ADD CONSTRAINT list_webhook_subscription_list_id_fkey
    FOREIGN KEY (list_id) REFERENCES list(id) ON DELETE SET NULL;
    """)


def downgrade():
    op.execute("""
    ALTER TABLE list_webhook_subscription
    DROP CONSTRAINT list_webhook_subscription_list_id_fkey;

    ALTER TABLE list_webhook_subscription
    ALTER COLUMN list_id SET NOT NULL;

    ALTER TABLE list_webhook_subscription
    ADD CONSTRAINT list_webhook_subscription_list_id_fkey
    FOREIGN KEY (list_id) REFERENCES list(id) ON DELETE CASCADE;
    """)
