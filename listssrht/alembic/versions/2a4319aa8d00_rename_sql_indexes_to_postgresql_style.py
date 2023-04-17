"""Rename SQL indexes to PostgreSQL-style

Revision ID: 2a4319aa8d00
Revises: 7faa55b46247
Create Date: 2023-03-13 21:55:24.211402

"""

# revision identifiers, used by Alembic.
revision = '2a4319aa8d00'
down_revision = 'b4a0d2c6fc5a'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE "user" RENAME CONSTRAINT user_username_unique TO user_username_key;
    ALTER TABLE list RENAME CONSTRAINT uq_list_owner_id_name TO list_owner_id_name_key;
    ALTER TABLE access RENAME CONSTRAINT uq_access_list_id_email TO access_list_id_email_key;
    ALTER TABLE access RENAME CONSTRAINT uq_access_list_id_user_id TO access_list_id_user_id_key;
    ALTER TABLE email RENAME CONSTRAINT uq_email_list_message_id TO email_list_id_message_id_key;
	ALTER TABLE subscription RENAME CONSTRAINT subscription_list_id_email_unique TO subscription_list_id_email_key;
	ALTER TABLE subscription RENAME CONSTRAINT subscription_list_id_user_id_unique TO subscription_list_id_user_id_key;

    DROP INDEX ix_user_username;
    ALTER INDEX ix_patchset_tool_key RENAME TO patchset_tool_key_idx;
    """)

def downgrade():
    op.execute("""
    ALTER TABLE "user" RENAME CONSTRAINT user_username_key TO user_username_unique;
    ALTER TABLE list RENAME CONSTRAINT list_owner_id_name_key TO uq_list_owner_id_name;
    ALTER TABLE access RENAME CONSTRAINT access_list_id_email_key TO uq_access_list_id_email;
    ALTER TABLE access RENAME CONSTRAINT access_list_id_user_id_key TO uq_access_list_id_user_id;
    ALTER TABLE email RENAME CONSTRAINT email_list_id_message_id_key TO uq_email_list_message_id;
	ALTER TABLE subscription RENAME CONSTRAINT subscription_list_id_email_key TO subscription_list_id_email_unique;
	ALTER TABLE subscription RENAME CONSTRAINT subscription_list_id_user_id_key TO subscription_list_id_user_id_unique;

    CREATE INDEX ix_user_username ON "user" USING btree (username);
    ALTER INDEX patchset_tool_key_idx RENAME TO ix_patchset_tool_key;
    """)
