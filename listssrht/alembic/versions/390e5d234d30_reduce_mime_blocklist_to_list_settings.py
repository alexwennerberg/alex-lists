"""Reduce MIME blocklist to list settings

Revision ID: 390e5d234d30
Revises: f47b7cf45e4a
Create Date: 2024-11-12 10:47:51.667142

"""

# revision identifiers, used by Alembic.
revision = '390e5d234d30'
down_revision = 'f47b7cf45e4a'

from alembic import op
import sqlalchemy as sa
from srht.config import cfg

server_reject_mimetypes = cfg("lists.sr.ht::worker", "reject-mimetypes", default='')

def upgrade():
    # Make sure this is NULL rather than emtpy string for the SQL to work
    server_reject_mimetypes_ = 'NULL'
    if server_reject_mimetypes:
        server_reject_mimetypes_ = f"'{server_reject_mimetypes}'"
 
    op.execute("""
        ALTER TABLE list ALTER COLUMN reject_mimetypes SET DEFAULT 'text/html';
    """)
    op.execute(f"""
        UPDATE list
        SET reject_mimetypes = concat_ws(
            ',',
            VARIADIC array_append(
                string_to_array(reject_mimetypes, ','),
                {server_reject_mimetypes_}
            )
        )
        WHERE position({server_reject_mimetypes_} IN reject_mimetypes) = 0;
    """)


def downgrade():
    # Here, empty string is okay
    op.execute("""
        ALTER TABLE list ALTER COLUMN reject_mimetypes SET DEFAULT '';
    """)
    op.execute(f"""
        UPDATE list
        SET reject_mimetypes = btrim(
            replace(reject_mimetypes, '{server_reject_mimetypes}', ''),
            ','
        );
    """)
