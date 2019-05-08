import email
import io
import sqlalchemy as sa
from email import policy
from srht.database import Base
from unidiff import PatchSet

class Email(Base):
    __tablename__ = 'email'
    _no_autoupdate = True
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    subject = sa.Column(sa.Unicode(2048), nullable=False)
    message_id = sa.Column(sa.Unicode(2048), nullable=False)
    in_reply_to = sa.Column(sa.Unicode(2048))
    headers = sa.Column(sa.JSON, nullable=False)
    body = sa.Column(sa.Unicode, nullable=False)
    envelope = sa.Column(sa.Unicode, nullable=False)
    is_patch = sa.Column(sa.Boolean, nullable=False)
    """true if email is via git format-patch"""
    is_request_pull = sa.Column(sa.Boolean, nullable=False)
    """true if email is via git request-pull"""

    list_id = sa.Column(sa.Integer, sa.ForeignKey('list.id'))
    list = sa.orm.relationship('List', backref=sa.orm.backref('messages'))

    parent_id = sa.Column(sa.Integer, sa.ForeignKey('email.id'))
    replies = sa.orm.relationship('Email',
            backref=sa.orm.backref('parent',
                remote_side=[id]),
            foreign_keys=[parent_id])

    thread_id = sa.Column(sa.Integer, sa.ForeignKey('email.id'))
    descendants = sa.orm.relationship('Email',
            backref=sa.orm.backref('thread',
                remote_side=[id]),
            foreign_keys=[thread_id])

    nreplies = sa.Column(sa.Integer, server_default='0')
    nparticipants = sa.Column(sa.Integer, server_default='1')

    sender_id = sa.Column(sa.Integer, sa.ForeignKey('user.id'))
    sender = sa.orm.relationship('User',
            backref=sa.orm.backref('sent_messages'))

    # "[PATCH meta.sr.ht v2 1/4] Add thing to stuff"
    # patch_index: 1; patch_count: 4
    # patch_version: 2
    # patch_prefix: meta.sr.ht
    # patch_subject: Add thing to stuff
    patch_index = sa.Column(sa.Integer)
    patch_count = sa.Column(sa.Integer)
    patch_version = sa.Column(sa.Integer)
    patch_prefix = sa.Column(sa.Unicode)
    patch_subject = sa.Column(sa.Unicode)

    superseded_by_id = sa.Column(sa.Integer, sa.ForeignKey('email.id'))
    superseded_by = sa.orm.relationship('Email',
            backref=sa.orm.backref('previous_version', remote_side=[id]),
            foreign_keys=[superseded_by_id])

    patchset_id = sa.Column(sa.Integer, sa.ForeignKey('patchset.id'))
    patchset = sa.orm.relationship("Patchset",
            backref=sa.orm.backref("patches"), foreign_keys=[patchset_id])

    # TODO: Enumerate CC's and create a relationship there

    def __repr__(self):
        return '<Email {} {}>'.format(self.id, self.subject)

    def to_dict(self, short=False):
        return {
            "id": self.id,
            "created": self.created,
            "subject": self.subject,
            "message_id": self.message_id,
            "parent_id": self.parent_id,
            "thread_id": self.thread_id,
            "list": self.list.to_dict(short=True),
            "sender": self.sender.to_dict(short=True) if self.sender else None,
            **({
                "is_patch": self.is_patch,
                "is_request_pull": self.is_request_pull,
                "replies": self.nreplies,
                "participants": self.nparticipants,
                "envelope": self.envelope,
            } if not short else {})
        }

    def parsed(self):
        if hasattr(self, "_parsed"):
            return self._parsed
        self._parsed = email.message_from_string(
                self.envelope, policy=email.policy.SMTP)
        return self._parsed

    def patch(self):
        if not self.is_patch:
            return None
        if hasattr(self, "_patch"):
            return self._patch
        with io.StringIO(self.body) as f:
            self._patch = PatchSet(f)
        return self._patch
