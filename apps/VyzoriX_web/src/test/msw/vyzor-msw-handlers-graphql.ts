import { http, HttpResponse, delay } from 'msw';

const GraphQLEndpoint = '/:orgId/graphql';
const GraphQLBatchEndpoint = '/:orgId/graphql/batch';

type GraphQLHandler = (variables: Record<string, unknown>) => unknown;

const handlers: Record<string, GraphQLHandler> = {};

export function registerGraphQLResponse(operationName: string, handler: GraphQLHandler): void {
  handlers[operationName] = handler;
}

export function clearGraphQLResponses(): void {
  for (const key of Object.keys(handlers)) {
    delete handlers[key];
  }
}

function processOperation(body: {
  operationName?: string;
  query?: string;
  variables?: Record<string, unknown>;
}): unknown {
  const operationName = body.operationName ?? extractOperationName(body.query);
  const variables = body.variables ?? {};

  const handler = handlers[operationName ?? ''];
  if (!handler) {
    throw new Error(`No mock handler for GraphQL operation: ${operationName}`);
  }
  return handler(variables);
}

export function createGraphQLHandlers() {
  return [

    http.post(GraphQLEndpoint, async ({ request }) => {
      await delay(30);
      const body = (await request.json()) as {
        operationName?: string;
        query?: string;
        variables?: Record<string, unknown>;
      };

      try {
        const data = processOperation(body);
        return HttpResponse.json({ data });
      } catch (err) {
        return HttpResponse.json(
          { errors: [{ message: err instanceof Error ? err.message : 'Unknown error' }] },
          { status: 200 },
        );
      }
    }),

    http.post(GraphQLBatchEndpoint, async ({ request }) => {
      await delay(30);
      const body = (await request.json()) as Array<{
        operationName?: string;
        query?: string;
        variables?: Record<string, unknown>;
      }>;

      const results = body.map((op) => {
        try {
          const data = processOperation(op);
          return { data };
        } catch (err) {
          return { errors: [{ message: err instanceof Error ? err.message : 'Unknown error' }] };
        }
      });

      return HttpResponse.json(results);
    }),
  ];
}

function extractOperationName(query?: string): string | undefined {
  if (!query) return undefined;
  const match = query.match(/(?:query|mutation)\s+(\w+)/);
  return match?.[1];
}
