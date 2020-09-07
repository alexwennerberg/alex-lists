"""Add constraints to subscriptions

Revision ID: ed291e59b2a0
Revises: 41c629d9fb0f
Create Date: 2020-09-07 12:10:44.469865

"""

# revision identifiers, used by Alembic.
revision = 'ed291e59b2a0'
down_revision = '41c629d9fb0f'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.create_unique_constraint("subscription_list_id_email_unique",
            "subscription", ["list_id", "email"])
    op.create_unique_constraint("subscription_list_id_user_id_unique",
            "subscription", ["list_id", "user_id"])
    op.create_check_constraint("subscription_email_xor_user_id",
            "subscription", "(email IS NULL OR user_id IS NULL) " +
                "AND (email IS NOT NULL OR user_id IS NOT NULL)")


def downgrade():
    op.drop_constraint("subscription_list_id_email_unique")
    op.drop_constraint("subscription_list_id_user_id_unique")
    op.drop_constraint("subscription_email_xor_user_id")
