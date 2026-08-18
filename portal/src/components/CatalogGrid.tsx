'use client';

import React from 'react';
import { DemoTemplate } from '@/lib/types';
import { CatalogCard } from './CatalogCard';

interface CatalogGridProps {
  templates: DemoTemplate[];
  onLaunch: (template: DemoTemplate) => void;
}

export function CatalogGrid({ templates, onLaunch }: CatalogGridProps) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      {templates.map((tpl) => (
        <CatalogCard key={tpl.id} template={tpl} onLaunch={onLaunch} />
      ))}
    </div>
  );
}
