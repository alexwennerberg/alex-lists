"""Use canonical user ID

Revision ID: 53ee8f573895
Revises: 31fafe3a4a70
Create Date: 2022-07-14 15:07:05.396381

"""

# revision identifiers, used by Alembic.
revision = '53ee8f573895'
down_revision = '31fafe3a4a70'

from alembic import op
import sqlalchemy as sa


# These tables all have a column referencing "user"(id)
tables = [
    ("access", "user_id"),
    ("email", "sender_id"),
    ("gql_list_wh_sub", "user_id"),
    ("gql_user_wh_sub", "user_id"),
    ("list", "owner_id"),
    ("list_webhook_subscription", "user_id"),
    ("oauthtoken", "user_id"),
    ("subscription", "user_id"),
    ("user_webhook_subscription", "user_id"),
]

def upgrade():
    # Drop unique constraints
    op.execute("""
    ALTER TABLE access DROP CONSTRAINT uq_access_list_id_user_id;
    ALTER TABLE list DROP CONSTRAINT uq_list_owner_id_name;
    ALTER TABLE subscription DROP CONSTRAINT subscription_list_id_user_id_unique;
    """)

    # Drop foreign key constraints and update user IDs
    for (table, col) in tables:
        op.execute(f"""
        ALTER TABLE {table} DROP CONSTRAINT {table}_{col}_fkey;
        UPDATE {table} t SET {col} = u.remote_id FROM "user" u WHERE u.id = t.{col};
        """)

    # Update primary key
    op.execute("""
    ALTER TABLE "user" DROP CONSTRAINT user_pkey;
    ALTER TABLE "user" DROP CONSTRAINT user_remote_id_key;
    ALTER TABLE "user" RENAME COLUMN id TO old_id;
    ALTER TABLE "user" RENAME COLUMN remote_id TO id;
    ALTER TABLE "user" ADD PRIMARY KEY (id);
    ALTER TABLE "user" ADD UNIQUE (old_id);
    """)

    # Add foreign key constraints
    for (table, col) in tables:
        op.execute(f"""
        ALTER TABLE {table} ADD CONSTRAINT {table}_{col}_fkey FOREIGN KEY ({col}) REFERENCES "user"(id) ON DELETE CASCADE;
        """)

    # Add unique constraints
    op.execute("""
    ALTER TABLE access ADD CONSTRAINT uq_access_list_id_user_id UNIQUE (list_id, user_id);
    ALTER TABLE list ADD CONSTRAINT uq_list_owner_id_name UNIQUE (owner_id, name);
    ALTER TABLE subscription ADD CONSTRAINT subscription_list_id_user_id_unique UNIQUE (list_id, user_id);
    """)


def downgrade():
    # Drop unique constraints
    op.execute("""
    ALTER TABLE access DROP CONSTRAINT uq_access_list_id_user_id;
    ALTER TABLE list DROP CONSTRAINT uq_list_owner_id_name;
    ALTER TABLE subscription DROP CONSTRAINT subscription_list_id_user_id_unique;
    """)

    # Drop foreign key constraints and update user IDs
    for (table, col) in tables:
        op.execute(f"""
        ALTER TABLE {table} DROP CONSTRAINT {table}_{col}_fkey;
        UPDATE {table} t SET {col} = u.old_id FROM "user" u WHERE u.id = t.{col};
        """)

    # Update primary key
    op.execute("""
    ALTER TABLE "user" DROP CONSTRAINT user_pkey;
    ALTER TABLE "user" DROP CONSTRAINT user_old_id_key;
    ALTER TABLE "user" RENAME COLUMN id TO remote_id;
    ALTER TABLE "user" RENAME COLUMN old_id TO id;
    ALTER TABLE "user" ADD PRIMARY KEY (id);
    ALTER TABLE "user" ADD UNIQUE (remote_id);
    """)

    # Add foreign key constraints
    for (table, col) in tables:
        op.execute(f"""
        ALTER TABLE {table} ADD CONSTRAINT {table}_{col}_fkey FOREIGN KEY ({col}) REFERENCES "user"(id) ON DELETE CASCADE;
        """)

    # Add unique constraints
    op.execute("""
    ALTER TABLE access ADD CONSTRAINT uq_access_list_id_user_id UNIQUE (list_id, user_id);
    ALTER TABLE list ADD CONSTRAINT uq_list_owner_id_name UNIQUE (owner_id, name);
    ALTER TABLE subscription ADD CONSTRAINT subscription_list_id_user_id_unique UNIQUE (list_id, user_id);
    """)
