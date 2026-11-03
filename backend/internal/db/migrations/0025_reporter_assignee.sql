-- A project may send new issues to their reporter.
ALTER TABLE projects MODIFY default_assignee ENUM('unassigned', 'lead', 'reporter') NOT NULL DEFAULT 'unassigned';
