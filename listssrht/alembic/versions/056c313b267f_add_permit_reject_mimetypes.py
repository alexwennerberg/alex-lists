"""Add permit/reject mimetypes

Revision ID: 056c313b267f
Revises: 86fbf902f15b
Create Date: 2019-03-25 15:55:06.110157

"""

# revision identifiers, used by Alembic.
revision = '056c313b267f'
down_revision = '86fbf902f15b'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.add_column('list', sa.Column('permit_mimetypes',
        sa.Unicode, nullable=False,
        server_default="text/*,application/pgp-signature,application/pgp-keys"))
    op.add_column('list', sa.Column('reject_mimetypes',
        sa.Unicode, nullable=False, server_default=''))


def downgrade():
    op.delete_column('list', 'permit_mimetypes')
    op.delete_column('list', 'reject_mimetypes')
