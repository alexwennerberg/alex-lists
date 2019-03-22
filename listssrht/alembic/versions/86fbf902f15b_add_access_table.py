"""Add access table

Revision ID: 86fbf902f15b
Revises: 617471810f6f
Create Date: 2019-03-22 09:54:22.083985

"""

# revision identifiers, used by Alembic.
revision = '86fbf902f15b'
down_revision = '617471810f6f'

from alembic import op
import sqlalchemy as sa
from srht.flagtype import FlagType
from listssrht.types.listaccess import ListAccess

def upgrade():
    op.create_table('access',
        sa.Column('id', sa.Integer, primary_key=True),
        sa.Column('created', sa.DateTime, nullable=False),
        sa.Column('updated', sa.DateTime, nullable=False),
        sa.Column('email', sa.Unicode),
        sa.Column('user_id', sa.Integer, sa.ForeignKey('user.id')),
        sa.Column('list_id', sa.Integer,
                sa.ForeignKey('list.id', ondelete="CASCADE"), nullable=False),
        sa.Column('permissions', FlagType(ListAccess),
                nullable=False, server_default=str(ListAccess.all.value)))

def downgrade():
    op.drop_table('access')
