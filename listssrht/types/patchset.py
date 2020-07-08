import sqlalchemy as sa
import sqlalchemy_utils as sau
from enum import Enum
from srht.database import Base

class PatchsetStatus(Enum):
    proposed = "proposed"
    needs_revision = "needs_revision"
    superseded = "superseded"
    approved = "approved"
    rejected = "rejected"
    applied = "applied"

class Patchset(Base):
    __tablename__ = 'patchset'
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    subject = sa.Column(sa.Unicode(2048), nullable=False)
    prefix = sa.Column(sa.Unicode)
    version = sa.Column(sa.Integer, nullable=False)

    status = sa.Column(sau.ChoiceType(PatchsetStatus, impl=sa.String()),
            nullable=False, server_default="proposed")

    list_id = sa.Column(sa.Integer,
            sa.ForeignKey('list.id', ondelete="CASCADE"),
            nullable=False)
    list = sa.orm.relationship('List',
            backref=sa.orm.backref('patchsets'))

    cover_letter_id = sa.Column(sa.Integer, sa.ForeignKey('email.id'))
    cover_letter = sa.orm.relationship("Email", foreign_keys=[cover_letter_id])

    superseded_by_id = sa.Column(sa.Integer, sa.ForeignKey('patchset.id'))
    superseded_by = sa.orm.relationship('Patchset',
            backref=sa.orm.backref('previous_version', remote_side=[id]),
            foreign_keys=[superseded_by_id])

    def to_dict(self, short=True):
        return {
            "id": self.id,
            "created": self.created,
            "updated": self.updated,
            "subject": self.subject,
            "prefix": self.prefix,
            "version": self.version,
            "status": self.status.value,
            **({
                "list": self.list.to_dict(short=True),
                "cover_letter": self.cover_letter.to_dict(short=True)
                    if self.cover_letter else None,
                "superseded_by": self.superseded_by.to_dict(short=True)
                    if self.superseded_by else None,
            } if not short else {}),
        }
