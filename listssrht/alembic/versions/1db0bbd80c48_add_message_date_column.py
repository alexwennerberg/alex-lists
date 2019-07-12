"""Add message date column

Revision ID: 1db0bbd80c48
Revises: 223cd3247273
Create Date: 2019-07-12 19:07:36.556698

"""

# revision identifiers, used by Alembic.
revision = '1db0bbd80c48'
down_revision = '223cd3247273'

import email
from email.utils import parsedate_to_datetime
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
    message_date = sa.Column(sa.DateTime)

def upgrade():
    op.add_column('email',
            sa.Column('message_date', sa.DateTime))
    bind = op.get_bind()
    session = Session(bind=bind)
    for mail in session.query(Email).all():
        envelope = email.message_from_string(
                mail.envelope, policy=policy.default)
        date = envelope.get("Date")
        if date is not None:
            mail.message_date = parsedate_to_datetime(date)
    session.commit()


def downgrade():
    op.drop_column('email', 'message_date')
