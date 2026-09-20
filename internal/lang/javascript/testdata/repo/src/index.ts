import { helper } from './util.js';
import { Button } from './components';
import { add } from '@app/lib/math';
import React from 'react';
import { createRoot } from 'react-dom/client';
import { readFileSync } from 'node:fs';
import * as path from 'path';
import tool from '@scope/tool/sub';
import chalk from 'chalk';
import { shared } from '@acme/shared';
import './styles.css';
import '../outside';
import alias from '~/alias';
export { thing } from './util';

export function main(): void {}
export class App {
  render() { return null; }
}
export interface Props { a: number }
export type T = string;
export enum E { A }
export const handler = () => {};
const X = 1;
