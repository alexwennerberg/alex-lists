"""Make gql_list_wh_sub.list_id on delete cascade

Revision ID: 6197b8a2e537
Revises: f7a8c3e239ce
Create Date: 2025-02-21 16:58:01.583446

"""

# revision identifiers, used by Alembic.
revision = '6197b8a2e537'
down_revision = 'f7a8c3e239ce'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    DELETE FROM gql_list_wh_sub
    WHERE list_id IS NULL;
    """)
    op.execute("""
    ALTER TABLE gql_list_wh_sub
    DROP CONSTRAINT gql_list_wh_sub_list_id_fkey,
    ADD CONSTRAINT gql_list_wh_sub_list_id_fkey
    FOREIGN KEY (list_id) REFERENCES "list"(id) ON DELETE CASCADE;
    """)


def downgrade():
    op.execute("""
    ALTER TABLE gql_list_wh_sub
    DROP CONSTRAINT gql_list_wh_sub_list_id_fkey,
    ADD CONSTRAINT gql_list_wh_sub_list_id_fkey
    FOREIGN KEY (list_id) REFERENCES "list"(id) ON DELETE SET NULL;
    """)
