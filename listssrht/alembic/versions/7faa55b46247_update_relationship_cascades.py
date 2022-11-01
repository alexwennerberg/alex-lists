"""Update relationship cascades

Revision ID: 7faa55b46247
Revises: 53ee8f573895
Create Date: 2022-11-01 14:26:40.942228

"""

# revision identifiers, used by Alembic.
revision = '7faa55b46247'
down_revision = '53ee8f573895'

from alembic import op
import sqlalchemy as sa

cascades = [
    ("list", "user", "owner_id", "CASCADE"),
    ("list", "list", "mirror_id", "SET NULL"),
    ("access", "user", "user_id", "CASCADE"),
    ("email", "email", "parent_id", "SET NULL"),
    ("email", "email", "thread_id", "SET NULL"),
    ("email", "user", "sender_id", "SET NULL"),
    ("email", "email", "superseded_by_id", "SET NULL"),
    ("patchset", "email", "cover_letter_id", "SET NULL"),
    ("patchset", "patchset", "superseded_by_id", "SET NULL"),
    ("subscription", "user", "user_id", "CASCADE"),
    ("gql_user_wh_sub", "user", "user_id", "CASCADE"),
    ("gql_list_wh_sub", "user", "user_id", "CASCADE"),
    ("oauthtoken", "user", "user_id", "CASCADE"),
    ("list_webhook_subscription", "user", "user_id", "CASCADE"),
    ("list_webhook_subscription", "oauthtoken", "token_id", "CASCADE"),
    ("user_webhook_subscription", "user", "user_id", "CASCADE"),
    ("user_webhook_subscription", "oauthtoken", "token_id", "CASCADE"),
    ("user_webhook_delivery", "user_webhook_subscription", "subscription_id", "CASCADE"),
]

def upgrade():
    for (table, relation, col, do) in cascades:
        op.execute(f"""
        ALTER TABLE {table} DROP CONSTRAINT IF EXISTS {table}_{col}_fkey;
        ALTER TABLE {table} ADD CONSTRAINT {table}_{col}_fkey
            FOREIGN KEY ({col})
            REFERENCES "{relation}"(id) ON DELETE {do};
        """)


def downgrade():
    for (table, relation, col, do) in tables:
        op.execute(f"""
        ALTER TABLE {table} DROP CONSTRAINT IF EXISTS {table}_{col}_fkey;
        ALTER TABLE {table} ADD CONSTRAINT {table}_{col}_fkey FOREIGN KEY ({col}) REFERENCES "{relation}"(id);
        """)
