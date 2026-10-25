"""Exercise deployment decisions with fake Docker; never contacts a server."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class AutoUpdateTest(unittest.TestCase):
    def run_update(self, mode):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            binary = root / 'bin'
            binary.mkdir()
            docker = binary / 'docker'
            docker.write_text('''#!/usr/bin/env python3
import os, sys
from pathlib import Path
args = sys.argv[1:]
mode = os.environ['TEST_MODE']
with open(os.environ['TEST_LOG'], 'a') as log:
    log.write(' '.join(args) + '\\n')
if args[0] == 'inspect':
    print('sha256:old-api' if args[-1] == 'handynotion-api' else 'sha256:old-web')
elif args[:2] == ['image', 'inspect']:
    if 'Labels' in args[3]:
        print('' if mode == 'unlabelled' else ('b' * 40 if mode == 'mismatch' and 'web' in args[-1] else 'a' * 40))
    else:
        name = 'api' if 'api' in args[-1] else 'web'
        print('sha256:' + ('old-' if mode == 'current' else 'new-') + name)
elif args[0] == 'exec':
    if mode == 'backup-failed':
        sys.exit(1)
    print('backup data ' * 100)
elif args[0] == 'compose':
    override = Path(args[args.index('-f', 3) + 1]).read_text()
    if mode == 'deploy-failed' and 'new-api' in override:
        sys.exit(1)
''')
            docker.chmod(0o755)
            for name in ['flock', 'curl']:
                path = binary / name
                path.write_text('#!/bin/sh\nexit 0\n')
                path.chmod(0o755)
            script = Path(__file__).with_name('auto-update.sh').read_text()
            script = script.replace('export PATH=', f'export PATH={binary}:')
            script = script.replace('/run/lock/handy-notion-update.lock', str(root / 'lock'))
            script = script.replace('/myproject/handy-notion', str(root))
            script = script.replace('/var/backups/handy-notion', str(root / 'backups'))
            script = script.replace('/run/handy-notion-images.', str(root / 'override.'))
            target = root / 'update.sh'
            target.write_text(script)
            log = root / 'calls'
            result = subprocess.run(['bash', str(target)], env={**os.environ, 'TEST_MODE': mode, 'TEST_LOG': str(log)}, capture_output=True, text=True)
            return result, log.read_text(), list((root / 'backups').glob('*.sql.gz'))

    def test_no_change(self):
        result, calls, _ = self.run_update('current')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('compose ', calls)

    def test_unpaired_or_unlabelled_images_are_not_deployed(self):
        for mode in ['unlabelled', 'mismatch']:
            with self.subTest(mode=mode):
                result, calls, _ = self.run_update(mode)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertNotIn('compose ', calls)
                self.assertNotIn('exec postgres', calls)

    def test_paired_images_backed_up_and_deployed(self):
        result, calls, backups = self.run_update('success')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(backups), 1)
        self.assertEqual(calls.count('compose '), 1)
        self.assertLess(calls.index('exec postgres'), calls.index('compose '))
        self.assertIn('--no-deps --pull never api web', calls)

    def test_failed_backup_blocks_deploy(self):
        result, calls, backups = self.run_update('backup-failed')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('compose ', calls)
        self.assertEqual(backups, [])

    def test_failed_deploy_attempts_rollback(self):
        result, calls, _ = self.run_update('deploy-failed')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls.count('compose '), 2)
        self.assertIn('previous images', result.stderr)


if __name__ == '__main__':
    unittest.main()