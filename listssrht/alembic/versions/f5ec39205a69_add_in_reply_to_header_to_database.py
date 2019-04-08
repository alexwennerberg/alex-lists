"""Add in_reply_to header to database

Revision ID: f5ec39205a69
Revises: 056c313b267f
Create Date: 2019-04-08 12:20:44.929749

"""

# revision identifiers, used by Alembic.
revision = 'f5ec39205a69'
down_revision = '056c313b267f'

import email
from email import policy
from alembic import op
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, Session as BaseSession, relationship
import sqlalchemy as sa

Session = sessionmaker()
Base = declarative_base()

class Email(Base):
    __tablename__ = 'email'
    id = sa.Column(sa.Integer, primary_key=True)
    envelope = sa.Column(sa.Unicode, nullable=False)
    in_reply_to = sa.Column(sa.Unicode(2048), nullable=False)


def upgrade():
    op.add_column('email', sa.Column('in_reply_to', sa.Unicode(2048)))

    bind = op.get_bind()
    session = Session(bind=bind)
    for mail in session.query(Email).all():
        envelope = email.message_from_string(
                mail.envelope, policy=policy.SMTP)
        in_reply_to = envelope["In-Reply-To"]
        mail.in_reply_to = in_reply_to
    session.commit()


def downgrade():
    op.drop_column('email', 'in_reply_to')
