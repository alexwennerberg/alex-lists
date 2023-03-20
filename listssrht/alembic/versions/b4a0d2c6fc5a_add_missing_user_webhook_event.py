"""Add missing user webhook event

Revision ID: b4a0d2c6fc5a
Revises: 7faa55b46247
Create Date: 2023-03-20 13:38:15.888787

"""

# revision identifiers, used by Alembic.
revision = 'b4a0d2c6fc5a'
down_revision = '7faa55b46247'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TYPE "webhook_event" ADD VALUE 'PATCHSET_RECEIVED';
    """)


def downgrade():
    op.execute("""
    ALTER TYPE "webhook_event" RENAME TO "webhook_event_old";
    CREATE TYPE "webhook_event" AS ENUM (
        'LIST_CREATED',
        'LIST_UPDATED',
        'LIST_DELETED',
        'EMAIL_RECEIVED'
    );
    ALTER TABLE "gql_user_wh_sub" ALTER COLUMN events TYPE varchar[];
    ALTER TABLE "gql_user_wh_sub" ALTER COLUMN events TYPE webhook_event[] USING events::webhook_event[];
    ALTER TABLE "gql_user_wh_delivery" ALTER COLUMN event TYPE varchar;
    ALTER TABLE "gql_user_wh_delivery" ALTER COLUMN event TYPE webhook_event USING event::webhook_event;
    DROP TYPE "webhook_event_old";
    """)
