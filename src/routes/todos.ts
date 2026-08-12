import { Router } from 'express';
import { z } from 'zod';
import { prisma } from '../prisma';

export const todosRouter = Router();

const createSchema = z.object({
  title: z.string().min(1).max(200),
  done: z.boolean().optional(),
});

const updateSchema = z.object({
  title: z.string().min(1).max(200).optional(),
  done: z.boolean().optional(),
});

const idParam = z.coerce.number().int().positive();

// GET /api/todos
todosRouter.get('/', async (_req, res, next) => {
  try {
    const todos = await prisma.todo.findMany({ orderBy: { id: 'desc' } });
    res.json(todos);
  } catch (err) {
    next(err);
  }
});

// GET /api/todos/:id
todosRouter.get('/:id', async (req, res, next) => {
  try {
    const id = idParam.parse(req.params.id);
    const todo = await prisma.todo.findUnique({ where: { id } });
    if (!todo) return res.status(404).json({ error: 'Todo not found' });
    res.json(todo);
  } catch (err) {
    next(err);
  }
});

// POST /api/todos
todosRouter.post('/', async (req, res, next) => {
  try {
    const data = createSchema.parse(req.body);
    const todo = await prisma.todo.create({ data });
    res.status(201).json(todo);
  } catch (err) {
    next(err);
  }
});

// PATCH /api/todos/:id
todosRouter.patch('/:id', async (req, res, next) => {
  try {
    const id = idParam.parse(req.params.id);
    const data = updateSchema.parse(req.body);
    const existing = await prisma.todo.findUnique({ where: { id } });
    if (!existing) return res.status(404).json({ error: 'Todo not found' });
    const todo = await prisma.todo.update({ where: { id }, data });
    res.json(todo);
  } catch (err) {
    next(err);
  }
});

// DELETE /api/todos/:id
todosRouter.delete('/:id', async (req, res, next) => {
  try {
    const id = idParam.parse(req.params.id);
    const existing = await prisma.todo.findUnique({ where: { id } });
    if (!existing) return res.status(404).json({ error: 'Todo not found' });
    await prisma.todo.delete({ where: { id } });
    res.status(204).send();
  } catch (err) {
    next(err);
  }
});
