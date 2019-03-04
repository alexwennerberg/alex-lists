"""Add webhook tables 

Revision ID: c3c2bea8d735
Revises: 84d51b97a028
Create Date: 2019-03-04 13:24:13.352424

"""

# revision identifiers, used by Alembic.
revision = 'c3c2bea8d735'
down_revision = '84d51b97a028'

from alembic import op
import sqlalchemy as sa
import sqlalchemy_utils as sau


def upgrade():
    op.create_table('list_webhook_subscription',
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("created", sa.DateTime, nullable=False),
        sa.Column("url", sa.Unicode(2048), nullable=False),
        sa.Column("events", sa.Unicode, nullable=False),
        sa.Column("user_id", sa.Integer, sa.ForeignKey("user.id")),
        sa.Column("token_id", sa.Integer, sa.ForeignKey("oauthtoken.id")),
        sa.Column("list_id", sa.Integer, sa.ForeignKey("list.id")),
    )
    op.create_table('list_webhook_delivery',
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("uuid", sau.UUIDType, nullable=False),
        sa.Column("created", sa.DateTime, nullable=False),
        sa.Column("event", sa.Unicode(256), nullable=False),
        sa.Column("url", sa.Unicode(2048), nullable=False),
        sa.Column("payload", sa.Unicode(65536), nullable=False),
        sa.Column("payload_headers", sa.Unicode(16384), nullable=False),
        sa.Column("response", sa.Unicode(65536)),
        sa.Column("response_status", sa.Integer, nullable=False),
        sa.Column("response_headers", sa.Unicode(16384)),
        sa.Column("subscription_id", sa.Integer,
            sa.ForeignKey('list_webhook_subscription.id'), nullable=False),
    )
    op.create_table('user_webhook_subscription',
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("created", sa.DateTime, nullable=False),
        sa.Column("url", sa.Unicode(2048), nullable=False),
        sa.Column("events", sa.Unicode, nullable=False),
        sa.Column("user_id", sa.Integer, sa.ForeignKey("user.id")),
        sa.Column("token_id", sa.Integer, sa.ForeignKey("oauthtoken.id")),
    )
    op.create_table('user_webhook_delivery',
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("uuid", sau.UUIDType, nullable=False),
        sa.Column("created", sa.DateTime, nullable=False),
        sa.Column("event", sa.Unicode(256), nullable=False),
        sa.Column("url", sa.Unicode(2048), nullable=False),
        sa.Column("payload", sa.Unicode(65536), nullable=False),
        sa.Column("payload_headers", sa.Unicode(16384), nullable=False),
        sa.Column("response", sa.Unicode(65536)),
        sa.Column("response_status", sa.Integer, nullable=False),
        sa.Column("response_headers", sa.Unicode(16384)),
        sa.Column("subscription_id", sa.Integer,
            sa.ForeignKey('user_webhook_subscription.id'), nullable=False),
    )


def downgrade():
    op.drop_table('list_webhook_subscription')
    op.drop_table('list_webhook_delivery')
    op.drop_table('user_webhook_subscription')
    op.drop_table('user_webhook_delivery')
