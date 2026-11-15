// Setup of the browser test project: the components' tests see the app's
// styles, and each test starts from an empty page.
import { afterEach } from 'vitest';
import { cleanup } from './render';
import './app.css';

afterEach(cleanup);
